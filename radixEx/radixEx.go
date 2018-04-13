// radixEx project radixEx.go
package radixEx

import "wujun/radixtree"

type node struct {
	subtree *radixtree.Tree
	data    interface{}
	id      int
}

type Tree struct {
	root  *radixtree.Tree
	idgen int
}

type Env struct {
	keys   []string
	matchs []string
	bmatch bool
	id     int
	count  int
}

type Walkfn func(v interface{}, e *Env) bool
type ExKey []string

func New() *Tree {
	rt := &Tree{
		root: radixtree.New(),
	}
	return rt
}

func Compile(src string) (key ExKey) {
	key = make(ExKey, 0, 100)
	last := 0
	i := 0
	for i = range src {
		if src[i] == '%' {
			if last != i {
				key = append(key, src[last:i])

			}
			key = append(key, "")
			last = i + 1
		}
	}
	if last < i {
		key = append(key, src[last:i])
	}
	return
}

func (tr *Tree) Insert(keys []string, value interface{}) (old interface{}, b bool) {

	if tr.root == nil {
		tr.root = radixtree.New()
	}

	curtree := tr.root
	var ref *node
	for _, v := range keys {
		tr.idgen++
		ref = &node{id: tr.idgen}
		oldnode, _ := curtree.Insert(v, ref)
		if oldnode != nil {
			ref.subtree = oldnode.(*node).subtree
			ref.data = oldnode.(*node).data
			ref.id = oldnode.(*node).id
		}

		if ref.subtree == nil {
			ref.subtree = radixtree.New()
		}
		curtree = ref.subtree
	}

	if ref.data != nil {
		old = ref.data
		b = true
	}

	ref.data = value

	return
}

func (tr *Tree) WalkPath(key string, f Walkfn) {

	e := &Env{
		keys:   make([]string, 0, 100),
		matchs: make([]string, 0, 100),
	}
	tr.root.WalkPath(key, getCallback(tr.root, 0, key, f, e))

}

func getCallback(curtree *radixtree.Tree, pos int, search string, f Walkfn, e *Env) radixtree.WalkFn {
	e.count++
	return func(key string, v interface{}) bool {

		e.keys = append(e.keys, key)
		e.matchs = append(e.matchs, key)
		defer func() {
			e.keys = e.keys[:len(e.keys)-1]
			e.matchs = e.matchs[:len(e.matchs)-1]
		}()
		rt := false
		nextnode := v.(*node)

		pos1 := pos + len(key)

		nextkey := search[pos1:]

		if len(key) == 0 {
			for i := 0; i < len(nextkey); i++ {
				//for i := len(nextkey) - 1; i >= 0; i-- {
				//fmt.Println(pos1, i)
				e.matchs = append(e.matchs, nextkey[:i])
				nextnode.subtree.WalkPath(nextkey[i:], getCallback(nextnode.subtree, pos1+i, search, f, e))
				e.matchs = e.matchs[:len(e.matchs)-1]
				if e.bmatch {
					return true

				}
			}
		} else {

			if nextnode.subtree != nil {
				nextnode.subtree.WalkPath(nextkey, getCallback(nextnode.subtree, pos1, search, f, e))
			}

		}

		if nextnode.data != nil {
			if f != nil {
				if len(key) == 0 {
					e.matchs = append(e.matchs, search[pos:])
				}
				e.id = nextnode.id
				rt = f(nextnode.data, e)
				if len(key) == 0 {
					e.matchs = e.matchs[:len(e.matchs)-1]
				}
				if rt {
					e.bmatch = true
				}

			}
		}
		return rt
	}
}
