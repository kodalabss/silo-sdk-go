package compiler

type Node interface {
	node()
}

type Manifest struct {
	Version int
	Entries []Entry
}

func (m *Manifest) node() {}

type Entry struct {
	Name       string
	IsVariable bool
	Children   []Entry
	Intents    []Intent
	Line       int
}

func (e *Entry) node() {}

type Intent struct {
	Name   string
	Params map[string]interface{}
	Line   int
}

func (i *Intent) node() {}
