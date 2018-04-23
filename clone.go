package html

import (
	"golang.org/x/net/html"
)

func Clone(src *html.Node) *html.Node {

	cache := make(map[*html.Node]*html.Node)
	var cloneFn func(src *html.Node, cache map[*html.Node]*html.Node) *html.Node
	cloneFn = func(src *html.Node, cache map[*html.Node]*html.Node) *html.Node {
		if src == nil {
			return nil
		}

		if val, ok := cache[src]; ok {
			return val
		}

		val := &html.Node{}

		cache[src] = val // Put in cache first
		val.Parent = cloneFn(src.Parent, cache)
		val.FirstChild = cloneFn(src.FirstChild, cache)
		val.LastChild = cloneFn(src.LastChild, cache)
		val.PrevSibling = cloneFn(src.PrevSibling, cache)
		val.NextSibling = cloneFn(src.NextSibling, cache)

		val.Type = src.Type
		val.DataAtom = src.DataAtom
		val.Data = src.Data
		val.Namespace = src.Namespace

		for _, v := range src.Attr {
			val.Attr = append(val.Attr, v)
		}
		return val
	}
	x := cloneFn(src, cache)
	x.Parent, x.PrevSibling, x.NextSibling = nil, nil, nil
	return x
}
