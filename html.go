package html

import (
	"errors"
	"fmt"

	css "github.com/andybalholm/cascadia"
	log "github.com/golang/glog"
	"golang.org/x/net/html"

	"thep.uk/tools/parse"

	"bytes"
	"reflect"
	"strings"
)

// syntax is text|data|attr.$val
// e.g. `css:"fieldX,text" or  `css:"fieldX,attr.href"
// if the option (text/attr/data) is ommited "text" directive is applied(by default)
var tagParser = parse.NewTagParser("@")

type Tag struct {
	parse.Tag
	Omitempty     bool
	OmitemptyAttr bool
	Attribute     string // if attribute is not nil
}

func parseTag(s string) (Tag, error) {
	tag := Tag{}
	tag.Tag = tagParser.Parse(s, "css")
	if tag.Opt == "" {
		tag.Opt = "text"
	}
	switch {
	case tag.Opt == "omitempty": // it's a struct with omitempty
		tag.Omitempty = true
	case strings.HasPrefix(tag.Opt, "attr."):
		tag.Attribute = strings.TrimPrefix(tag.Opt, "attr.")
		switch {
		case tag.Attribute == "":
			return Tag{}, errors.New("Invalid tag option " + s)
		case strings.HasSuffix(tag.Attribute, "@omitemptyAttr"):
			tag.Attribute = strings.TrimSuffix(tag.Attribute, "@omitemptyAttr")
			tag.OmitemptyAttr = true
		case strings.HasSuffix(tag.Attribute, "@omitempty"):
			tag.Attribute = strings.TrimSuffix(tag.Attribute, "@omitempty")
			tag.Omitempty = true
		}
	}
	return tag, nil
}

func Marshal(n *html.Node, v interface{}) error {
	//log.Errorf("Encode %#v", v)
	//log.Flush()
	//log.Errorf("v is %#v", v)
	rv := reflect.ValueOf(v)
	typ := rv.Type()
	if typ.Kind() == reflect.Ptr {
		rv = rv.Elem()
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return errors.New("only structs are supported")
	}
	if typ.PkgPath() == "reflect" {
		return fmt.Errorf("Invalid value..., package is reflect?")
	}
	for i := 0; i < typ.NumField(); i++ {
		//log.Errorf("field %v", i)
		//log.Flush()
		fi := typ.Field(i)
		fv := rv.Field(i)
		ft := fi.Type
		var tag Tag
		var err error
		if fi.Tag != "" {
			tag, err = parseTag(string(fi.Tag))
			if err != nil {
				return err
			}
		}
		if ft.Kind() == reflect.Ptr {
			ft = ft.Elem()
			if fv.IsNil() {
				if !tag.Omitempty {
					log.Errorf("Not omitempty raw %v, tag %#v", fi.Tag, tag)
					continue
				}
				if tag.Name != "" {
					if err := removeField(n, tag.Name); err != nil {
						log.Error(err)
						return err
					}
					continue
				}
				log.Errorf("tag.Name %v", tag.Name)
				if tag.Omitempty && tag.Name == "" && ft.Kind() == reflect.Struct {
					removeFields(ft, n)
				}
				continue
			}
			fv = fv.Elem()
		}

		if string(fi.Tag) == "" {
			if fi.Type.Kind() == reflect.Struct {
				if err := Marshal(n, fv.Interface()); err != nil {
					return err
				}
			} else {
				//log.Infof("Field %v , kind %s is skipped, missing or invalid tag? raw %s",
				//	fi.Name, fi.Type.Kind(), fi.Tag)
			}
			continue
		}
		if tag.Name == "" && tag.Opt == "" {
			err := fmt.Errorf("Field has invalid tag %q, type %q.%q, field %v",
				fi.Tag, typ.PkgPath(), typ.Name(), i)
			log.Error(err)
			return err
		}

		//log.Errorf("tag.Opt %v, tag.name %v", tag.Opt, tag.Name)
		//log.Flush()
		var matchA []*html.Node
		// self
		if tag.Name == "_" {
			matchA = []*html.Node{n}
		} else {
			//match, err = firstMatch(n, tag.Name)
			matchA, err = allMatch(n, tag.Name)
			if err != nil {
				log.Error(err)
				return err
			}
		}
		for k := range matchA {
			if err := marshalField(n, matchA[k], ft, fv, tag); err != nil {
				log.Error(err)
				return err
			}
		}
	}
	return nil
}
func allMatch(n *html.Node, s string) ([]*html.Node, error) {
	sel, err := css.Compile(s)
	if err != nil {
		return nil, errors.New("selector invalid: " + string(s) + ": " + err.Error())
	}
	match := sel.MatchAll(n)
	if match == nil {
		//	log.Errorf("N %#v", n)
		//	log.Flush()
		buf := bytes.NewBuffer(nil)
		html.Render(buf, n)
		return nil, errors.New("sels " + s +
			": not found, html: " + buf.String())
	}
	return match, nil
}

func firstMatch(n *html.Node, s string) (*html.Node, error) {
	sel, err := css.Compile(s)
	if err != nil {
		return nil, errors.New("selector invalid: " + string(s) + ": " + err.Error())
	}
	match := sel.MatchFirst(n)
	if match == nil {
		//	log.Errorf("N %#v", n)
		//	log.Flush()
		buf := bytes.NewBuffer(nil)
		html.Render(buf, n)
		return nil, errors.New("sels " + s +
			": not found, html: " + buf.String())
	}
	return match, nil
}

func marshalField(n, match *html.Node, ft reflect.Type, fv reflect.Value, tag Tag) error {
	switch ft.Kind() {
	default:
		if err := setField(n, match, tag, fv); err != nil {
			log.Error(err)
			return err
		}
	case reflect.Bool:
		if fv.Interface().(bool) == false {
			if tag.Omitempty {
				removeField(n, tag.Name)
				return nil
			}
			if tag.OmitemptyAttr {
				removeAttribute(n, tag.Name, tag.Attribute)
				return nil
			}
		}
		if err := setField(n, match, tag, fv); err != nil {
			log.Error(err)
			return err
		}
	case reflect.String:
		if fv.Interface().(string) == "" {
			if tag.Omitempty {
				removeField(n, tag.Name)
				return nil
			}
			if tag.OmitemptyAttr {
				removeAttribute(n, tag.Name, tag.Attribute)
				return nil
			}
		}
		if err := setField(n, match, tag, fv); err != nil {
			log.Error(err)
			return err
		}
	case reflect.Struct:
		if err := Marshal(match, fv.Interface()); err != nil {
			return err
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < fv.Len(); i++ {
			v := fv.Index(i)
			clone := Clone(match)
			if err := Marshal(clone, v.Interface()); err != nil {
				return err
			}
			//n.InsertBefore(clone, match.FirstChild)
			//match.Parent.AppendChild(clone)
			match.Parent.InsertBefore(clone, match)
		}
		// remove template / initial values?
		match.Parent.RemoveChild(match)
	}
	return nil
}
func setField(n, match *html.Node, tag Tag, fv reflect.Value) error {
	//log.Errorf("Kind is %v", ft.Kind())
	data := fmt.Sprintf("%v", fv.Interface())
	switch {
	case tag.Opt == "data":
		n.Data = data
	case tag.Opt == "text":
		//n.Data = data
		if !setText(match, data) {
			return errors.New("sels " + tag.Name + " text children")
		}
	case tag.Attribute != "":
		if tag.Omitempty && data == "" {
			log.Errorf("Omitempty  %v ", tag.Attribute)
			return nil
		}
		var ok bool
		for k := range match.Attr {
			if match.Attr[k].Key == tag.Attribute {
				switch tag.Attribute {
				case "autofocus":
				default:
					match.Attr[k].Val = data
				}

				ok = true
				break
			}
		}
		if !ok {
			attr := html.Attribute{Key: tag.Attribute}
			switch tag.Attribute {
			case "autofocus":
			default:
				attr.Val = data
			}
			match.Attr = append(match.Attr, attr)
		}
	default:
		err := fmt.Errorf("Invalid tag option %#v", tag)
		log.Error(err)
		return err
	}
	return nil
}

func removeField(doc *html.Node, tagName string) error {
	log.Errorf("Remove field %v", tagName)
	match, err := firstMatch(doc, tagName)
	if err != nil {
		log.Error(err)
		return err
	}
	match.Parent.RemoveChild(match)
	return nil
}
func removeAttribute(doc *html.Node, tagName, attribute string) error {
	log.Errorf("Remove attribute %v, attribute ", tagName, attribute)
	match, err := firstMatch(doc, tagName)
	if err != nil {
		log.Error(err)
		return err
	}
	for k, v := range match.Attr {
		if v.Key == attribute {
			log.Flush()
			match.Attr = append(match.Attr[:k], match.Attr[k+1:]...)
			log.Errorf("match - Remove attribute %v, attribute , got %#v",
				tagName, attribute, match.Attr)
			log.Flush()
			break
		}
	}
	return nil
}

// removes all the nodes matching the fields of typ
func removeFields(typ reflect.Type, doc *html.Node) {
	log.Infoln("not implemented")
}

// setText is a helper which is used to set the data of the inner node
func setText(n *html.Node, s string) bool {
	if n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
		n.FirstChild.Data = s
		return true
	}
	return false
}
