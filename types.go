package main

import (
    "fmt"
)

// some notes:
// for now, a process is not itself an expression
// an expression is only ever a number, a var, or a list
type expression interface {
    PrintExpression() string
}

type variable int64

func (v variable) PrintExpression() string {
    return fmt.Sprintf("v#%d", v)
}

type number int64

func (n number) PrintExpression() string {
    return fmt.Sprintf("%d", n)
}

type special uint8

const (
    emptylist special = iota
)

func (s special) PrintExpression() string {
    if s == emptylist {
        return "[]"
    }
    panic(fmt.Sprintf("unknown special builtin %d", s))
}

type list struct {
    head expression // can be anything
    tail expression // has to be list or emptylist!
}

func (l list) PrintExpression() string {
    if l.tail == emptylist {
        return fmt.Sprintf("[%s]", l.head.PrintExpression())
    }
    return fmt.Sprintf("[%s,%s]", l.head.PrintExpression(), l.tail.PrintExpression())
}

type process struct {
    functor string
    args []expression
}

func (p process) arity() int {
    return len(p.args)
}

func (p process) isPredefined() bool {
    // only predefined functors for now
    return p.functor == ":=" || p.functor == "isplus"
}

type rule struct {
    head process
    //guard []guard
    body []process
}

type bindings map[variable]expression

