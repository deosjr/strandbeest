package main

import (
    "fmt"
    "math/rand/v2"
    "sync"
)

/*
An interpreter consists of the following goroutines:
- the main interpreter routine, adding processes to pool
and listening to all of the results of reduce, updating bindings
- numWorkers worker routines running reduce in parallel
*/

type Interpreter struct {
    sync.Mutex
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

func NewSingleThreadedInterpreter(program []rule) *Interpreter {
    return &Interpreter{
        program: program,
    }
}

func (i *Interpreter) interpretSinglethreaded(initial []process) bindings {
    globalBindings := bindings{}
    for _, p := range initial {
        i.putProcess(p)
    }
    for len(i.pool) > 0 {
        p := i.getProcess()
        if p.isPredefined() {
            theta, suspend := i.execute(globalBindings, p)
            if suspend {
                i.pool = append(i.pool, p)
                continue
            }
            for k, v := range theta {
                globalBindings[k] = v 
            }
            continue
        }
        rules := i.getPossibleRules(p)
        theta, r1, ok := i.reduce(globalBindings, p, rules)
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
}

type result struct {
    b bindings
    p process
    body []process
    success bool
}

func (i *Interpreter) interpret(initial []process) bindings {
    inCh := make(chan work, i.numWorkers)
    outCh := make(chan result, i.numWorkers)
    globalBindings := bindings{}
    for n:=0; n<i.numWorkers; n++ {
        go i.workReduce(inCh, outCh)
    }
    for _, p := range initial {
        i.putProcess(p)
    }
    // todo: deadlock detection
    workInProgress := 0
    for {
        if len(i.pool) == 0 {
            // no more work to schedule
            if workInProgress == 0 {
                // and not awaiting any scheduled work: we are done
                close(inCh)
                close(outCh)
                break
            }
            // await work result
            result := <-outCh
            i.handleResult(globalBindings, result)
            workInProgress--
            continue
        }
        // try to get work result, otherwise schedule more work
        select {
        case result := <-outCh:
            i.handleResult(globalBindings, result)
            workInProgress--
        default:
            p := i.getProcess()
            if p.isPredefined() {
                theta, suspend := i.execute(globalBindings, p)
                if suspend {
                    i.putProcess(p)
                    continue
                }
                outCh <- result{b: theta, p:p, success:true}
                workInProgress++
                continue
            }
            // todo: think about how to pass bindings around
            // possible race condition: 
            // - one reduce starts reading from bindings
            // - handling process updates bindings halfway
            // conclusion: have to somehow pass copies/nested references
            // lets start with ugly/slow map copies and go from there
            // note: this race can still happen! handler will have to check and reject solutions?
            b := copyBindings(globalBindings)
            inCh <- work{b, p}
            workInProgress++
        }
    }
    return globalBindings
}

func (i *Interpreter) handleResult(globalBindings bindings, res result) {
    if !res.success {
        i.putProcess(res.p)
        return
    }
    for k := range res.b {
        if _, ok := globalBindings[k]; ok {
            // single-assignment means if we find a clash, we return the work
            i.putProcess(res.p)
            return
        }
    }
    for k, v := range res.b {
        globalBindings[k] = v
    }
    for _, r := range res.body {
        i.putProcess(r)
    }
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

func (i *Interpreter) execute(b bindings, p process) (bindings, bool) {
    newb := bindings{}
    switch p.functor {
    case ":=":
        // X := Y   % assign Y to X in global bindings
        // todo: validation, occurs checks, etc..
        // what if first arg is not a var?
        x := walk(b, p.args[0]).(variable)
        y := walk(b, p.args[1])
        newb[x] = y
    case "isplus":
        // isplus(X,Y,Z)    % X is Y + Z
        // todo: this might fail/suspend?
        // what if first arg is not a var?
        x := walk(b, p.args[0]).(variable)
        y := walk(b, p.args[1])
        z := walk(b, p.args[2])
        if _, isNum := y.(number); !isNum {
            return nil, true
        }
        if _, isNum := z.(number); !isNum {
            return nil, true
        }
        newb[x] = number(y.(number) + z.(number))
    default:
        panic(fmt.Sprintf("unknown predefined process %s", p.functor))
    }
    return newb, false
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

func (i *Interpreter) workReduce(inCh <-chan work, outCh chan<- result) {
    for w := range inCh {
        rules := i.getPossibleRules(w.p)
        theta, r1, ok := i.reduce(w.b, w.p, rules)
        if !ok {
            outCh <- result{p:w.p, success:false}
            continue
        }
        outCh <- result{b:theta, p:w.p, body:r1.body, success:true}
    }
}

func (i *Interpreter) reduce(b bindings, p process, rules []rule) (bindings, rule, bool) {
    rand.Shuffle(len(rules), func(i, j int) {
	    rules[i], rules[j] = rules[j], rules[i]
    })
    for _, r := range rules {
        r1 := i.freshCopy(r)
        m, ok := cmatch(b, p, r1)
        if !ok {
            continue
        }
        return m, r1, true
    }
    return nil, rule{}, false
}

func (i *Interpreter) fresh() variable {
    i.Lock()
    v := variable(i.varcounter)
    i.varcounter += 1
    i.Unlock()
    return v
}

// replace each variable in the rule template with a fresh unbound var
func (i *Interpreter) freshCopy(r rule) rule {
    b := bindings{}
    body := make([]process, len(r.body))
    for n:=0; n<len(r.body); n++ {
        body[n] = i.replaceFresh(b, r.body[n])
    }
    return rule{ head: i.replaceFresh(b, r.head), body: body }
}

func (i *Interpreter) replaceFresh(b bindings, p process) process {
    args := make([]expression, len(p.args))
    for n:=0; n<len(p.args); n++ {
        args[n] = i.replaceFreshExp(b, p.args[n])
    }
    return process{ functor: p.functor, args: args }
}

func (i *Interpreter) replaceFreshExp(b bindings, e expression) expression {
    if v, ok := e.(variable); ok {
        if ev, alreadyReplaced := b[v]; alreadyReplaced {
            return ev
        }
        newv := i.fresh()
        b[v] = newv
        return newv
    }
    if l, ok := e.(list); ok {
        return list{
            head: i.replaceFreshExp(b, l.head),
            tail: i.replaceFreshExp(b, l.tail),
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
