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
	varcounter  int64
	numWorkers  int
	program     []rule
	bindings    bindings
	pool        sync.Pool
	suspensions map[variable][]process
}

// program is assumed static, ie no dynamic rule assertions
func NewInterpreter(program []rule, numWorkers int) *Interpreter {
	return &Interpreter{
		numWorkers:  numWorkers,
		program:     program,
		bindings:    bindings{},
		suspensions: map[variable][]process{},
	}
}

func NewSingleThreadedInterpreter(program []rule) *Interpreter {
	return &Interpreter{
		program:     program,
		bindings:    bindings{},
		suspensions: map[variable][]process{},
	}
}

// returns bindings and boolean=true if deadlock detected
func (i *Interpreter) interpretSinglethreaded(initial []process) (bindings, bool) {
	for _, p := range initial {
		i.putProcess(p)
	}
	for {
		p, ok := i.getProcess()
		if !ok {
			break
		}
		if p.isPredefined() {
			theta, ok, suspendOn := i.execute(i.bindings, p)
			if !ok {
				if len(suspendOn) == 0 {
					// if no suspensions, this process is guaranteed to never succeed
					// don't put the process back into the pool
					continue
				}
				// suspend processes until one of vars are bound
				for _, v := range suspendOn {
					i.suspensions[v] = append(i.suspensions[v], p)
				}
				continue
			}
			i.commitBindings(i.bindings, theta)
			continue
		}
		rules := i.getPossibleRules(p)
		ok, theta, r1, suspendOn := i.reduce(i.bindings, p, rules)
		if !ok {
			if len(suspendOn) == 0 {
				// if no suspensions, this process is guaranteed to never succeed
				// don't put the process back into the pool
				continue
			}
			// suspend processes until one of vars are bound
			for _, v := range suspendOn {
				i.suspensions[v] = append(i.suspensions[v], p)
			}
			continue
		}
		i.commitBindings(i.bindings, theta)
		for _, p := range r1.body {
			i.putProcess(p)
		}
	}
	if len(i.suspensions) > 0 {
		return nil, true
	}
	return i.bindings, false
}

// NOTE: these 3 are only called from main interpreter routine, or there will be trouble!
func (i *Interpreter) commitBindings(b, theta bindings) {
	for k, v := range theta {
		b[k] = v
		// todo: make sure not to put processes back multiple times!
		if list, ok := i.suspensions[k]; ok {
			delete(i.suspensions, k)
			for _, p := range list {
				i.putProcess(p)
			}
		}
	}
}

func (i *Interpreter) putProcess(p process) {
	i.pool.Put(&p)
}

func (i *Interpreter) getProcess() (process, bool) {
	p := i.pool.Get()
	if p == nil {
		return process{}, false
	}
	return *p.(*process), true
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

type work struct {
	b bindings
	p process
}

type result struct {
	b         bindings
	p         process
	body      []process
	success   bool
	suspendOn []variable
}

func (i *Interpreter) interpret(initial []process) bindings {
	inCh := make(chan work, i.numWorkers)
	outCh := make(chan result, i.numWorkers)
	globalBindings := bindings{}
	for n := 0; n < i.numWorkers; n++ {
		go i.workReduce(inCh, outCh)
	}
	for _, p := range initial {
		i.putProcess(p)
	}
	// todo: deadlock detection
	workInProgress := 0
	for {
		p, ok := i.getProcess()
		if !ok {
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
			i.putProcess(p)
			i.handleResult(globalBindings, result)
			workInProgress--
		default:
			if p.isPredefined() {
				theta, ok, suspendOn := i.execute(globalBindings, p)
				if !ok {
					if len(suspendOn) == 0 {
						i.putProcess(p)
						continue
					}
					// todo
				}
				outCh <- result{b: theta, p: p, success: true}
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
	i.commitBindings(globalBindings, res.b)
	for _, r := range res.body {
		i.putProcess(r)
	}
}

// returns updates, bool indicating success, and which vars to suspend on if any
func (i *Interpreter) execute(b bindings, p process) (bindings, bool, []variable) {
	newb := bindings{}
	switch p.functor {
	case ":=":
		// X := Y   % assign Y to X in global bindings
		// todo: validation, occurs checks, etc..
		x := walk(b, p.args[0])
		xvar, ok := x.(variable)
		if !ok {
			panic(fmt.Sprintf("expected variable but got %s", x.PrintExpression()))
		}
		y := walk(b, p.args[1])
		newb[xvar] = y
	case "isplus":
		// isplus(X,Y,Z)    % X is Y + Z
		x := walk(b, p.args[0])
		xvar, ok := x.(variable)
		if !ok {
			panic(fmt.Sprintf("expected variable but got %s", x.PrintExpression()))
		}
		y := walk(b, p.args[1])
		z := walk(b, p.args[2])
		var suspensions []variable
		if yvar, yok := y.(variable); yok {
			suspensions = append(suspensions, yvar)
		}
		if zvar, zok := z.(variable); zok {
			suspensions = append(suspensions, zvar)
		}
		if len(suspensions) > 0 {
			return nil, false, suspensions
		}
		if _, isNum := y.(number); !isNum {
			return nil, false, nil
		}
		if _, isNum := z.(number); !isNum {
			return nil, false, nil
		}
		newb[xvar] = number(y.(number) + z.(number))
	default:
		panic(fmt.Sprintf("unknown predefined process %s", p.functor))
	}
	return newb, true, nil
}

func (i *Interpreter) workReduce(inCh <-chan work, outCh chan<- result) {
	for w := range inCh {
		rules := i.getPossibleRules(w.p)
		ok, theta, r1, sus := i.reduce(w.b, w.p, rules)
		if !ok {
			outCh <- result{p: w.p, success: false, suspendOn: sus}
			continue
		}
		outCh <- result{b: theta, p: w.p, body: r1.body, success: true}
	}
}

func (i *Interpreter) reduce(b bindings, p process, rules []rule) (bool, bindings, rule, []variable) {
	rand.Shuffle(len(rules), func(i, j int) {
		rules[i], rules[j] = rules[j], rules[i]
	})
	m := map[variable]struct{}{}
Loop:
	for _, r := range rules {
		r1 := i.freshCopy(r)
		ok, updates, sus := cmatch(b, p, r1)
		if !ok {
			if len(sus) == 0 {
				continue
			}
			for _, v := range sus {
				m[v] = struct{}{}
			}
			continue
		}
		guardsSucceed := true
		for _, g := range r1.guard {
			ok, sus := guardMatch(b, updates, g)
			if !ok {
				guardsSucceed = false
				if len(sus) == 0 {
					continue Loop
				}
				for _, v := range sus {
					m[v] = struct{}{}
				}
			}
		}
		if guardsSucceed {
			return true, updates, r1, nil
		}
	}
	var suspend []variable
	for k := range m {
		suspend = append(suspend, k)
	}
	return false, nil, rule{}, suspend
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
	head := i.replaceFresh(b, r.head)
	guards := make([]guard, len(r.guard))
	for n := 0; n < len(r.guard); n++ {
		guards[n] = guard{operator: r.guard[n].operator, args: []expression{
			i.replaceFreshExp(b, r.guard[n].args[0]),
			i.replaceFreshExp(b, r.guard[n].args[1]),
		}}
	}
	body := make([]process, len(r.body))
	for n := 0; n < len(r.body); n++ {
		body[n] = i.replaceFresh(b, r.body[n])
	}
	return rule{head: head, guard: guards, body: body}
}

func (i *Interpreter) replaceFresh(b bindings, p process) process {
	args := make([]expression, len(p.args))
	for n := 0; n < len(p.args); n++ {
		args[n] = i.replaceFreshExp(b, p.args[n])
	}
	return process{functor: p.functor, args: args}
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
// returns success boolean, updated bindings, and list vars to suspend on if any
func cmatch(base bindings, p process, r rule) (bool, bindings, []variable) {
	updates := bindings{}
	m := map[variable]struct{}{}
	for i := 0; i < p.arity(); i++ {
		success, suspend := unify(base, updates, p.args[i], r.head.args[i])
		if !success {
			if len(suspend) == 0 {
				return false, nil, nil
			}
			for _, v := range suspend {
				m[v] = struct{}{}
			}
		}
	}
	if len(m) == 0 {
		return true, updates, nil
	}
	var suspend []variable
	for k := range m {
		suspend = append(suspend, k)
	}
	return false, updates, suspend
}

// returns success boolean and list vars to suspend on if any
func guardMatch(base, updates bindings, g guard) (bool, []variable) {
	u := walk(base, walk(updates, g.args[0]))
	v := walk(base, walk(updates, g.args[1]))
	// guard args have to be fully instantiated, otherwise suspend
	var suspend []variable
	if uvar, ok := u.(variable); ok {
		suspend = append(suspend, uvar)
	}
	if vvar, ok := v.(variable); ok {
		suspend = append(suspend, vvar)
	}
	if len(suspend) > 0 {
		return false, suspend
	}
	switch g.operator {
	case Equal:
		return u == v, nil
	case NotEqual:
		return u != v, nil
	}
	panic("unknown operator in guard match")
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
// returns a success boolean and a list of variables on which to suspend, if any
func unify(base, updates bindings, u, v expression) (bool, []variable) {
	if u == underscore || v == underscore {
		return true, nil
	}
	u = walk(base, walk(updates, u))
	v = walk(base, walk(updates, v))
	if u == v {
		return true, nil
	}
	// variables in the rule head match anything
	if vvar, ok := v.(variable); ok {
		updates[vvar] = u
		return true, nil
	}
	// data-flow synchronization: if we have a var on the left, we should suspend
	if uvar, ok := u.(variable); ok {
		return false, []variable{uvar}
	}
	// remember, emptylist is a special case!
	uList, uIsList := u.(list)
	vList, vIsList := v.(list)
	if uIsList && vIsList {
		p, susp := unify(base, updates, uList.head, vList.head)
		q, susq := unify(base, updates, uList.tail, vList.tail)
		if p && q {
			return true, nil
		}
		if susp == nil || susq == nil {
			return false, nil
		}
		m := map[variable]struct{}{}
		for _, v := range susp {
			m[v] = struct{}{}
		}
		for _, v := range susq {
			m[v] = struct{}{}
		}
		merged := []variable{}
		for k := range m {
			merged = append(merged, k)
		}
		return false, merged
	}
	return false, nil
}

func copyBindings(b bindings) bindings {
	m := bindings{}
	for k, v := range b {
		m[k] = v
	}
	return m
}
