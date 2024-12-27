package main

import (
    "fmt"
    "strings"
    "strconv"
    "unicode"
)

var syntaxError = fmt.Errorf("syntax error")

// parseRule returns a rule, amount of tokens parsed, and error
// variables in rules are numbered by first occurence, starting at 0
// actual vars will be assigned during copying of a matched rule with fresh vars
func parseRule(tokens []string) (rule, int, error) {
    b := map[string]variable{}
    head, n, err := parseProcess(b, tokens)
    if err != nil {
        return rule{}, 0, err
    }
    if tokens[n] != ":-" || len(tokens) < n+1 {
        return rule{}, 0, syntaxError
    }
    consumed := n+1
    body := []process{}
    for {
        r, n, err := parseProcess(b, tokens[consumed:])
        if err != nil {
            return rule{}, 0, err
        }
        body = append(body, r)
        consumed += n
        if tokens[consumed] == "." {
            return rule{head:head, body:body}, consumed+1, nil
        }
        if tokens[consumed] != "," {
            return rule{}, 0, syntaxError
        }
        consumed++
    }
}

// parseProcess returns a process, amount of tokens parsed, and error
func parseProcess(b map[string]variable, tokens []string) (process, int, error) {
    if len(tokens) < 2 {
        return process{}, 0, syntaxError
    }
    functor := tokens[0]
    if tokens[1] != "(" {
        return process{}, 0, syntaxError
    }
    consumed := 2
    args := []expression{}
    for {
        e, n, err := parseExpression(b, tokens[consumed:])
        if err != nil {
            return process{}, 0, err
        }
        args = append(args, e)
        consumed += n
        if tokens[consumed] == ")" {
            return process{functor:functor, args:args}, consumed+1, nil
        }
        if tokens[consumed] != "," {
            return process{}, 0, syntaxError
        }
        consumed++
    }
}

// parseExpression returns an expression, amount of tokens parsed, and error
func parseExpression(b map[string]variable, tokens []string) (expression, int, error) {
    if len(tokens[0]) == 0 {
        return nil, 0, syntaxError
    }
    c := rune(tokens[0][0])
    if c == '[' {
        return parseList(b, tokens)
    }
    if unicode.IsNumber(c) {
        return parseNumber(tokens[0])
    }
    if unicode.IsUpper(c) {
        return parseVariable(b, tokens[0])
    }
    return nil, 0, syntaxError    
}

func parseNumber(s string) (number, int, error) {
    n, err := strconv.ParseInt(s, 10, 64)
    if err != nil {
        return number(0), 0, err
    }
    return number(n), 1, nil
}

func parseVariable(b map[string]variable, s string) (variable, int, error) {
    if v, ok := b[s]; ok {
        return v, 1, nil
    }
    b[s] = variable(len(b))
    return b[s], 1, nil
}

func parseList(b map[string]variable, tokens []string) (expression, int, error) {
    if tokens[0] == "[" && tokens[1] == "]" {
        return emptylist, 2, nil
    }
    head := []expression{}
    consumed := 1
    for {
        h, n, err := parseExpression(b, tokens[consumed:])
        if err != nil {
            return nil, 0, syntaxError
        }
        head = append(head, h)
        consumed += n
        if tokens[consumed] != "," {
            break 
        }
        consumed++
    }
    if tokens[consumed] == "]" {
        return makeList(head, emptylist), consumed+1, nil
    }
    if tokens[consumed] == "|" {
        tail, n, err := parseExpression(b, tokens[consumed+1:])
        if err != nil {
            return nil, 0, syntaxError
        }
        consumed += n+1
        if tokens[consumed] != "]" {
            return nil, 0, syntaxError
        }
        return makeList(head, tail), consumed+1, nil
    }
    return nil, 0, syntaxError
}

func makeList(head []expression, tail expression) expression {
    out := tail
    for i:=len(head)-1; i>=0; i-- {
        out = list{head:head[i], tail:out}
    }
    return out
}

func tokenize(s string) []string {
    out := []string{}
    s = strings.TrimSpace(s)
    for len(s) > 0 {
        switch s[:1] {
        case "(", ")", "[", "]", "|", ",", ".":
            out = append(out, s[:1])
            s = s[1:]
            s = strings.TrimSpace(s)
            continue
        }
        if len(s) > 1 && s[:2] == ":-" {
            out = append(out, s[:2])
            s = s[2:]
            s = strings.TrimSpace(s)
            continue
        }
        i := strings.IndexAny(s, "(),|]")
        out = append(out, s[:i])
        s = s[i:]
        s = strings.TrimSpace(s)
    }
    return out
}
