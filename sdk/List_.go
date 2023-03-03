package sdk

// List extends `[]interface{}`, and it should be used to represent json-like data.
type List []interface{}

func (l *List) do(do func()) {
	if len(*l) < 1 {
		*l = make(List, 0)
	}

	if do != nil {
		do()
	}
}

// Add new element.
func (l *List) Add(v ...interface{}) {
	l.do(func() {
		for _, v := range v {
			*l = append(*l, v)
		}
	})
}

// Delete an element.
func (l *List) Delete(k int) {
	l.do(func() {
		if k < len(*l) {
			copy((*l)[k:], (*l)[k+1:])
			(*l)[len(*l)-1] = ""
			*l = (*l)[:len(*l)-1]
		}
	})
}

// Map is an alternative of for loop.
func (l *List) Map(fn func(k int, v interface{})) {
	l.do(func() {
		if fn != nil {
			for k, v := range *l {
				fn(k, v)
			}
		}
	})
}
