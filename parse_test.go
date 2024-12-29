package main

import (
    "reflect"
    "testing"
)

func TestParseExpression(t *testing.T) {
    for i, tt := range []struct{
        b map[string]variable
        tokens []token
        want expression
        wantN int
        err error
    }{
        {
            tokens: []token{""},
            err: syntaxError{"not enough tokens to parse expression"},
        },
        {
            tokens: []token{"3"},
            want:   number(3),
            wantN:  1,
        },
        {
            tokens: []token{"L"},
            want:   variable(0),
            wantN:  1,
        },
        {
            tokens: []token{"[", "]"},
            want:   emptylist,
            wantN:  2,
        },
        {
            tokens: []token{"[", "42", "]"},
            want:   list{head: number(42), tail: emptylist},
            wantN:  3,
        },
        {
            tokens: []token{"[", "2", ",", "3", "]"},
            want:   list{head: number(2), tail:list{head:number(3), tail: emptylist}},
            wantN:  5,
        },
        {
            tokens: []token{"[", "X", "|", "Xs", "]"},
            want:   list{head: variable(0), tail: variable(1)},
            wantN:  5,
        },
    }{
        if tt.b == nil {
            tt.b = map[string]variable{}
        }
        got, gotN, err := parseExpression(tt.b, tt.tokens)
        if err != tt.err {
            t.Errorf("%d: got %v want %v", i, err, tt.err)
            continue
        }
        if got != tt.want {
            t.Errorf("%d: got %v want %v", i, got, tt.want)
        }
        if gotN != tt.wantN {
            t.Errorf("%d: got %v want %v", i, gotN, tt.wantN)
        }
    }
}

func TestParseProcess(t *testing.T) {
    for i, tt := range []struct{
        b map[string]variable
        tokens []token
        want process
        wantN int
        err error
    }{
        {
            tokens: []token{""},
            err: syntaxError{"not enough tokens to parse process"},
        },
        {
            tokens: []token{"foo", "(", "3", ")"},
            want:   process{functor:"foo", args:[]expression{number(3)}},
            wantN:  4,
        },
        {
            tokens: []token{"sum", "(", "[", "1", "|", "L", "]", ",", "R", ")"},
            want:   process{functor:"sum", args:[]expression{
                list{head:number(1), tail:variable(0)}, variable(1),
            }},
            wantN:  10,
        },
        {
            tokens: []token{":=", "(", "L", ",", "[", "2", ",", "3", "]", ")"},
            want:   process{functor:":=", args:[]expression{
                variable(0), list{head:number(2), tail:list{head:number(3), tail:emptylist}},
            }},
            wantN:  10,
        },
        {
            tokens: []token{"L", ":=", "42"},
            want:   process{functor:":=", args:[]expression{
                variable(0), number(42),
            }},
            wantN:  3,
        },
        {
            tokens: []token{"isplus", "(", "A1", ",", "A", ",", "1", ")"},
            want:   process{functor:"isplus", args:[]expression{
                variable(0), variable(1), number(1),
            }},
            wantN:  8,
        },
    }{
        if tt.b == nil {
            tt.b = map[string]variable{}
        }
        got, gotN, err := parseProcess(tt.b, tt.tokens)
        if err != tt.err {
            t.Errorf("%d: got %v want %v", i, err, tt.err)
            continue
        }
        if !reflect.DeepEqual(got, tt.want) {
            t.Errorf("%d: got %v want %v", i, got, tt.want)
        }
        if gotN != tt.wantN {
            t.Errorf("%d: got %v want %v", i, gotN, tt.wantN)
        }
    }
}

func TestParseRule(t *testing.T) {
    for i, tt := range []struct{
        tokens []token
        want rule
        wantN int
        err error
    }{
        {
            tokens: []token{""},
            err: syntaxError{"not enough tokens to parse process"},
        },
        {
            tokens: []token{"sum", "(", "L", ",", "Sum", ")", ":-", "sum1", "(", "L", ",", "0", ",", "Sum", ")", "."},
            want:   rule{
                head: process{functor:"sum", args: []expression{
                    variable(0), variable(1),
                }},
                body: []process{
                    {functor:"sum1", args: []expression{
                        variable(0), number(0), variable(1),
                    }},
                },
            },
            wantN:  16,
        },
        {
            tokens: []token{"member", "(", "X", ",", "[", "X1", "|", "Rest", "]", ",", "R", ")", ":-", "X", "=\\=", "X1", "|", "member", "(", "X", ",", "Rest", ",", "R", ")", "."},
            want:   rule{
                head: process{functor:"member", args: []expression{
                    variable(0), list{head:variable(1), tail:variable(2)}, variable(3),
                }},
                guard: []guard{
                    {operator: NotEqual, args: []expression{variable(0), variable(1)}},
                },
                body: []process{
                    {functor:"member", args: []expression{
                        variable(0), variable(2), variable(3),
                    }},
                },
            },
            wantN:  26,
        },
    }{
        got, gotN, err := parseRule(tt.tokens)
        if err != tt.err {
            t.Errorf("%d: got %v want %v", i, err, tt.err)
            continue
        }
        if !reflect.DeepEqual(got, tt.want) {
            t.Errorf("%d: got %v want %v", i, got, tt.want)
        }
        if gotN != tt.wantN {
            t.Errorf("%d: got %v want %v", i, gotN, tt.wantN)
        }
    }
}

func TestTokenize(t *testing.T) {
    for i, tt := range []struct{
        input string
        want []token
    }{
        {
            input: "",
            want : []token{},
        },
        {
            input: "sum(L,Sum) :- sum1(L,0,Sum).",
            want: []token{"sum", "(", "L", ",", "Sum", ")", ":-", "sum1", "(", "L", ",", "0", ",", "Sum", ")", "."},
        },
        {
            input: "L := [2, 3]",
            want : []token{"L", ":=", "[", "2", ",", "3", "]"},
        },
        {
            input: "A1 is A + X,",
            want : []token{"A1", "is", "A", "+", "X", ","},
        },
    }{
        got := tokenize(tt.input)
        if !reflect.DeepEqual(got, tt.want) {
            t.Errorf("%d: got %q want %q", i, got, tt.want)
        }
    }
}

