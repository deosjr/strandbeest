package main

import (
    "fmt"
    "strings"
)

type bindings map[variable]expression

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
    underscore
    true_value
    false_value
)

func (s special) PrintExpression() string {
    switch s {
    case emptylist: return "[]"
    case underscore: return "_"
    case true_value: return "true"
    case false_value: return "false"
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
    return fmt.Sprintf("[%s|%s]", l.head.PrintExpression(), l.tail.PrintExpression())
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
    return p.functor == ":=" || p.functor == "isplus" || p.functor == "is"
}

func (p process) isInfix() bool {
    return p.functor == ":=" || p.functor == "is"
}

func (p process) String() string {
    args := []string{}
    for _, arg := range p.args {
        args = append(args, arg.PrintExpression())
    }
    if p.isInfix() && len(args) == 2 {
        return fmt.Sprintf("%s %s %s", args[0], p.functor, args[1])
    }
    return fmt.Sprintf("%s(%s)", p.functor, strings.Join(args, ","))
}

type rule struct {
    head process
    guard []guard
    body []process
}

func (r rule) String() string {
    body := []string{}
    for _, p := range r.body {
        body = append(body, p.String())
    }
    if len(r.guard) == 0 {
        return fmt.Sprintf("%s :- %s.", r.head, strings.Join(body, ","))
    }
    guard := []string{}
    for _, g := range r.guard {
        guard = append(guard, g.String())
    }
    return fmt.Sprintf("%s :- %s | %s.", r.head, strings.Join(guard, ","), strings.Join(body, ","))
}

type guard struct {
    operator string
    args []expression
}

func (g guard) String() string {
    if len(g.args) == 1 {
        return fmt.Sprintf("%s(%s)", g.operator, g.args[0].PrintExpression())
    }
    return fmt.Sprintf("%s %s %s", g.args[0].PrintExpression(), g.operator, g.args[1].PrintExpression())
}
