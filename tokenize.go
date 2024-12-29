package main

import (
    "strings"
    "unicode"
)

type token string

const (
    OpenParen token = "("
    CloseParen = ")"
    OpenBracket = "["
    CloseBracket = "]"
    Commit = "|"
    Comma = ","
    Period = "."
    Underscore = "_"
    Turnstile = ":-"
    Assign = ":="
    Is = "is"
    Equal = "=="
    NotEqual = "=\\="
    True = "true"
    False = "false"
)

func (t token) IsNumber() bool {
    return unicode.IsNumber(rune(t[0]))
}

func (t token) IsVariable() bool {
    return unicode.IsUpper(rune(t[0]))
}

func (t token) IsSymbol() bool {
    return unicode.IsLower(rune(t[0]))
}

func (t token) IsOperator() bool {
    return t == Assign || t == Is
}

func (t token) IsGuard() bool {
    return t == Equal || t == NotEqual
}

func tokenize(s string) []token {
    out := []token{}
    s = strings.TrimSpace(s)
    for len(s) > 0 {
        var punct token
        switch s[:1] {
        case "(": punct = OpenParen
        case ")": punct = CloseParen
        case "[": punct = OpenBracket
        case "]": punct = CloseBracket
        case "|": punct = Commit
        case ",": punct = Comma
        case ".": punct = Period
        case "_": punct = Underscore
        }
        if len(punct) > 0 {
            out = append(out, punct)
            s = s[1:]
            s = strings.TrimSpace(s)
            continue
        }
        if len(s) > 2 {
            switch s[:3] {
            case "=\\=": punct = NotEqual
            }
            if len(punct) > 0 && strings.IndexAny(s, "\t\n (") == 3 {
                out = append(out, punct)
                s = s[3:]
                s = strings.TrimSpace(s)
                continue
            }
        }
        if len(s) > 1 {
            switch s[:2] {
            case ":-": punct = Turnstile
            case ":=": punct = Assign
            case "is": punct = Is
            case "==": punct = Equal
            }
            if len(punct) > 0 && strings.IndexAny(s, "\t\n (") == 2 {
                out = append(out, punct)
                s = s[2:]
                s = strings.TrimSpace(s)
                continue
            }
        }
        i := strings.IndexAny(s, "\t\n (),|].")
        out = append(out, token(s[:i]))
        s = s[i:]
        s = strings.TrimSpace(s)
    }
    return out
}
