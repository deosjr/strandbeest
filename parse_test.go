package main

import (
    "reflect"
    "testing"
)

func TestParseExpression(t *testing.T) {
    for i, tt := range []struct{
        b map[string]variable
        tokens []string
        want expression
        wantN int
        err error
    }{
        {
            tokens: []string{""},
            err: syntaxError,
        },
        {
            tokens: []string{"3"},
            want:   number(3),
            wantN:  1,
        },
        {
            tokens: []string{"L"},
            want:   variable(0),
            wantN:  1,
        },
        {
            tokens: []string{"[", "]"},
            want:   emptylist,
            wantN:  2,
        },
        {
            tokens: []string{"[", "42", "]"},
            want:   list{head: number(42), tail: emptylist},
            wantN:  3,
        },
        {
            tokens: []string{"[", "X", "|", "Xs", "]"},
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
        tokens []string
        want process
        wantN int
        err error
    }{
        {
            tokens: []string{""},
            err: syntaxError,
        },
        {
            tokens: []string{"foo", "(", "3", ")"},
            want:   process{functor:"foo", args:[]expression{number(3)}},
            wantN:  4,
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
        tokens []string
        want rule
        wantN int
        err error
    }{
        {
            tokens: []string{""},
            err: syntaxError,
        },
        {
            tokens: []string{"sum", "(", "L", ",", "Sum", ")", ":-", "sum1", "(", "L", ",", "0", ",", "Sum", ")", "."},
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
