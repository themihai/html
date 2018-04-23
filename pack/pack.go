package pack

import (
	"bytes"
	"fmt"
	css "github.com/andybalholm/cascadia"

	log "github.com/golang/glog"

	html2 "github.com/themihai/html"
	"golang.org/x/net/html"
	"io"
)

type FileReader interface {
	Open(path string) (io.ReadCloser, error)
}

func NewPacker(fr FileReader) Packer {
	return Packer{fr: fr}
}

func (p Packer) PackHTML(path string) (*html.Node, error) {
	doc, err := p.NewTPL(path)
	if err != nil {
		return nil, err
	}
	err = p.Pack(doc)
	return doc, err
}

type Packer struct {
	fr FileReader
}

func (p Packer) Pack(node *html.Node) error {
	sel, err := css.Compile(`import`)
	if err != nil {
		return err
	}
	imports := sel.MatchAll(node)
	if len(imports) == 0 {
		return nil
		//return fmt.Errorf("No imports statement")
	}
	for _, ptr := range imports {
		//log.Errorf("new imports")
		isHead := false
		for _, attr := range ptr.Attr {
			if attr.Key == "head" && attr.Val == "true" {
				isHead = true
				break
			}
		}
		//log.Errorf("isHead %v", isHead)
		for _, attr := range ptr.Attr {
			//	log.Errorf("new attribute")

			if attr.Key != "src" {
				continue
			}
			if attr.Val == "" {
				return fmt.Errorf("Invalid import source")
			}
			n, err := p.NewTPL(attr.Val)
			if err != nil {
				return fmt.Errorf("import %v: %v", attr.Val, err)
			}
			// pack it
			if err = p.Pack(n); err != nil {
				log.Error(err)
				return err
			}
			if isHead {
				sel, err = css.Compile("head")
			} else {
				sel, err = css.Compile("body")
			}
			if err != nil {
				return err
			}
			body := sel.MatchFirst(n)
			if body == nil {
				return fmt.Errorf("import %v: body  not found ", attr.Val)
			}
			next := body.FirstChild
			if next == nil {
				buf := bytes.NewBuffer(nil)
				parent := bytes.NewBuffer(nil)
				html.Render(buf, body)
				html.Render(buf, n)
				return fmt.Errorf("import %v: body has no children, content %s, parent %s",
					attr.Val, buf.String(), parent.String())
			}

			if isHead {
				ptr.AppendChild(html2.Clone(next))
			} else {
				ptr.Parent.InsertBefore(html2.Clone(next), ptr)
			}
			nexti := next
			for {
				if next == nil || next.NextSibling == nil {
					break
				}
				*next = *nexti.NextSibling
				*nexti = *next
				buf := bytes.NewBuffer(nil)
				html.Render(buf, next)
				//	log.Infof("Insert %s", buf.String())
				ptr.Parent.InsertBefore(html2.Clone(next), ptr)
				//log.Flush()
			}
		}
	}
	return nil
}

func (p Packer) NewTPL(path string) (*html.Node, error) {
	r, err := p.fr.Open(path)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	n, err := html.Parse(r)
	return n, err
}
