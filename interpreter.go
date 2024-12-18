package main

import (
    "fmt"
    "math/rand/v2"
)

/*
An interpreter consists of the following goroutines:
- the main interpreter routine, adding processes to pool
- the handler routine listening to all of the results of reduce and updating bindings
- the var routine issuing a constant buffer of fresh variables
- numWorkers worker routines running reduce in parallel
*/

type Interpreter struct {
    varcounter int64
    numWorkers int
    program []rule
    pool []process
}

// program is assumed static, ie no dynamic rule assertions
func NewInterpreter(program []rule, numWorkers int) *Interpreter {
    return &Interpreter{
        numWorkers: numWorkers,
        program: program,
    }
}

// todo: interpeter buffers new vars on a channel for global consumption
func (i *Interpreter) newVariable() variable {
    v := variable(i.varcounter)
    i.varcounter++
    return v
}

func (i *Interpreter) interpretSinglethreaded(initial []process) bindings {
    globalBindings := bindings{}
    for _, p := range initial {
        i.putProcess(p)
    }
    for len(i.pool) > 0 {
        p := i.getProcess()
        if p.isPredefined() {
            if ok := i.execute(globalBindings, p); !ok {
                i.pool = append(i.pool, p)
            }
            continue
        }
        rules := i.getPossibleRules(p)
        theta, r1, ok := reduce(globalBindings, p, rules)
        if !ok {
            i.pool = append(i.pool, p)
            continue
        }
        // commit to updates
        for k, v := range theta {
            globalBindings[k] = v
        }
        i.pool = append(i.pool, r1.body...)
    }
    return globalBindings
}

type work struct {
    b bindings
    p process
    rules []rule
}

type result struct {
    b bindings
    p process
    body []process
}

func (i *Interpreter) interpret(initial []process) bindings {
    inCh := make(chan work, i.numWorkers)
    outCh := make(chan result, i.numWorkers)
    //varCh := make(chan variable, i.numWorkers)
    //go handler(outCh)
    //go i.vars(varCh)
    globalBindings := bindings{}
    for n:=0; n<i.numWorkers; n++ {
        go workReduce(inCh, outCh)
    }
    for _, p := range initial {
        i.putProcess(p)
    }
    // todo: deadlock detection
    // todo: different halt condition, this might happen halfway work being done
    for len(i.pool) > 0 {
        p := i.getProcess()
        if p.isPredefined() {
            // todo: send the result of execute to handler?
            i.execute(globalBindings, p)
            continue
        }
        rules := i.getPossibleRules(p)
        // todo: think about how to pass bindings around
        // possible race condition: 
        // - one reduce starts reading from bindings
        // - handling process updates bindings halfway
        // conclusion: have to somehow pass copies/nested references
        // lets start with ugly/slow map copies and go from there
        // note: this race can still happen! handler will have to check and reject solutions?
        b := copyBindings(globalBindings)
        inCh <- work{b, p, rules}
    }
    return globalBindings
}

func (i *Interpreter) putProcess(p process) {
    i.pool = append(i.pool, p)
}

// todo: its either this, or shuffle i.pool each time we add to it?
func (i *Interpreter) getProcess() process {
    n := rand.IntN(len(i.pool))
    p := i.pool[n]
    if n == len(i.pool)-1 {
        i.pool = i.pool[:n]
    } else {
        i.pool = append(i.pool[:n], i.pool[n+1:]...)
    }
    return p
}

func (i *Interpreter) execute(b bindings, p process) bool {
    switch p.functor {
    case ":=":
        // X := Y   % assign Y to X in global bindings
        // todo: validation, occurs checks, etc..
        // what if first arg is not a var?
        x := walk(b, p.args[0]).(variable)
        y := walk(b, p.args[1])
        b[x] = y
    case "isplus":
        // isplus(X,Y,Z)    % X is Y + Z
        // todo: this might fail/suspend?
        // what if first arg is not a var?
        x := walk(b, p.args[0]).(variable)
        y := walk(b, p.args[1])
        z := walk(b, p.args[2])
        if _, isNum := y.(number); !isNum {
            return false
        }
        if _, isNum := z.(number); !isNum {
            return false
        }
        b[x] = number(y.(number) + z.(number))
    default:
        panic(fmt.Sprintf("unknown predefined process %s", p.functor))
    }
    return true
}

// as naive as possible; this can get optimised
func (i *Interpreter) getPossibleRules(p process) []rule {
    candidates := []rule{}
    for _, r := range i.program {
        if r.head.functor != p.functor {
            continue
        }
        if r.head.arity() != p.arity() {
            continue
        }
        candidates = append(candidates, r)
    }
    return candidates
}

func workReduce(inCh <-chan work, outCh chan<- result) {
    for w := range inCh {
        b, r, ok := reduce(w.b, w.p, w.rules)
        if !ok {
            // todo: return process to pool
            continue
        }
        var body []process = r.body
        //body := spawnBody(w.b, b, r)
        outCh <- result{b:b, p:w.p, body:body}
    }
}

func reduce(b bindings, p process, rules []rule) (bindings, rule, bool) {
    rand.Shuffle(len(rules), func(i, j int) {
	    rules[i], rules[j] = rules[j], rules[i]
    })
    for _, r := range rules {
        r1 := freshCopy(r)
        m, ok := cmatch(b, p, r1)
        if !ok {
            continue
        }
        return m, r1, true
    }
    return nil, rule{}, false
}

// todo: replace this with a proper 'global' var counter!
var vars int64 = 2 // starting vars in initial processes count
// replace each variable in the rule template with a fresh unbound var
func freshCopy(r rule) rule {
    b := bindings{}
    body := make([]process, len(r.body))
    for i:=0; i<len(r.body); i++ {
        body[i] = replaceFresh(b, r.body[i])
    }
    return rule{ head: replaceFresh(b, r.head), body: body }
}

func replaceFresh(b bindings, p process) process {
    args := make([]expression, len(p.args))
    for i:=0; i<len(p.args); i++ {
        args[i] = replaceFreshExp(b, p.args[i])
    }
    return process{ functor: p.functor, args: args }
}

func replaceFreshExp(b bindings, e expression) expression {
    if v, ok := e.(variable); ok {
        if ev, alreadyReplaced := b[v]; alreadyReplaced {
            return ev
        }
        newv := variable(vars)
        b[v] = newv
        vars++
        return newv
    }
    if l, ok := e.(list); ok {
        return list{
            head: replaceFreshExp(b, l.head),
            tail: replaceFreshExp(b, l.tail),
        }
    }
    return e
}

// assumes functor/arity already matching
// todo: check guards
func cmatch(base bindings, p process, r rule) (bindings, bool) {
    updates := bindings{}
    for i:=0; i<p.arity(); i++ {
        if !unify(base, updates, p.args[i], r.head.args[i]) {
            return nil, false
        }
    }
    return updates, true
}

func walk(b bindings, e expression) expression {
    v, ok := e.(variable)
    if !ok {
        return e
    }
    x, ok := b[v]
    if !ok {
        return v
    }
    return walk(b, x)
}

// unify reads from base bindings and adds to updates in place
func unify(base, updates bindings, u, v expression) bool {
    u = walk(updates, walk(base, u))
    v = walk(updates, walk(base, v))
    if u == v {
        return true
    }
    // variables in the rule head match anything
    if vvar, ok := v.(variable); ok {
        updates[vvar] = u
        return true
    }
    // data-flow synchronization: if we have a var on the left, we should suspend
    if _, ok := u.(variable); ok {
        return false
    }
    // remember, emptylist is a special case!
    uList, uIsList := u.(list)
    vList, vIsList := v.(list)
    if uIsList && vIsList {
        p := unify(base, updates, uList.head, vList.head)
        q := unify(base, updates, uList.tail, vList.tail)
        return p && q
    }
    return false
}

func copyBindings(b bindings) bindings {
    m := bindings{}
    for k, v := range b {
        m[k] = v
    }
    return m
}
