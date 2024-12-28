package main

import (
    "strconv"
)

type syntaxError struct {
    msg string
}

func (e syntaxError) Error() string {
    return e.msg
}

// parseRule returns a rule, amount of tokens parsed, and error
// variables in rules are numbered by first occurence, starting at 0
// actual vars will be assigned during copying of a matched rule with fresh vars
func parseRule(tokens []token) (rule, int, error) {
    b := map[string]variable{}
    head, n, err := parseProcess(b, tokens)
    if err != nil {
        return rule{}, 0, err
    }
    if tokens[n] != Turnstile || len(tokens) < n+1 {
        return rule{}, 0, syntaxError{"expected turnstile"}
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
        if tokens[consumed] == Period {
            return rule{head:head, body:body}, consumed+1, nil
        }
        if tokens[consumed] != Comma {
            return rule{}, 0, syntaxError{"expected comma"}
        }
        consumed++
    }
}

// parseProcess returns a process, amount of tokens parsed, and error
func parseProcess(b map[string]variable, tokens []token) (process, int, error) {
    if len(tokens) < 2 {
        return process{}, 0, syntaxError{"not enough tokens to parse process"}
    }
    if tokens[1] != OpenParen {
        return parseInfix(b, tokens)
    }
    // parse normal process form: functor(arg0, arg1, ...)
    functor := string(tokens[0])
    if tokens[1] != OpenParen {
        return process{}, 0, syntaxError{"expected open parens"}
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
        if tokens[consumed] == CloseParen {
            return process{functor:functor, args:args}, consumed+1, nil
        }
        if tokens[consumed] != Comma {
            return process{}, 0, syntaxError{"expected comma"}
        }
        consumed++
    }
}

func parseInfix(b map[string]variable, tokens []token) (process, int, error) {
    if len(tokens) < 3 {
        return process{}, 0, syntaxError{"not enough tokens to parse infix"}
    }
    if !tokens[0].IsVariable() {
        return process{}, 0, syntaxError{"expected variable in arg0"}
    }
    arg0, n0, err := parseExpression(b, tokens)
    if err != nil {
        return process{}, 0, err
    }
    f := string(tokens[n0])
    arg1, n1, err := parseExpression(b, tokens[n0+1:])
    if err != nil {
        return process{}, 0, err
    }
    return process{functor:f, args:[]expression{arg0, arg1}}, n0+n1+1, nil
}

// parseExpression returns an expression, amount of tokens parsed, and error
func parseExpression(b map[string]variable, tokens []token) (expression, int, error) {
    if len(tokens[0]) == 0 {
        return nil, 0, syntaxError{"not enough tokens to parse expression"}
    }
    if tokens[0] == OpenBracket {
        return parseList(b, tokens)
    }
    if tokens[0].IsNumber() {
        return parseNumber(string(tokens[0]))
    }
    if tokens[0].IsVariable() {
        return parseVariable(b, string(tokens[0]))
    }
    return nil, 0, syntaxError{}
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

func parseList(b map[string]variable, tokens []token) (expression, int, error) {
    if tokens[0] == OpenBracket && tokens[1] == CloseBracket {
        return emptylist, 2, nil
    }
    head := []expression{}
    consumed := 1
    for {
        h, n, err := parseExpression(b, tokens[consumed:])
        if err != nil {
            return nil, 0, syntaxError{}
        }
        head = append(head, h)
        consumed += n
        if tokens[consumed] != Comma {
            break 
        }
        consumed++
    }
    if tokens[consumed] == CloseBracket {
        return makeList(head, emptylist), consumed+1, nil
    }
    if tokens[consumed] == Commit {
        tail, n, err := parseExpression(b, tokens[consumed+1:])
        if err != nil {
            return nil, 0, err
        }
        consumed += n+1
        if tokens[consumed] != CloseBracket {
            return nil, 0, syntaxError{"expected closing bracket"}
        }
        return makeList(head, tail), consumed+1, nil
    }
    return nil, 0, syntaxError{}
}

func makeList(head []expression, tail expression) expression {
    out := tail
    for i:=len(head)-1; i>=0; i-- {
        out = list{head:head[i], tail:out}
    }
    return out
}
