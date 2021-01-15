package main

import (
	"runtime/debug"
	"sync"
)

type ICallable = func(_data *DataTest) *ResultTest

type IBoolFunc = func(_data *DataTest) bool

type IPrepareFunc = func(_data *DataTest, input PrepareTest) *DataTest

type INodeBeginLogger = func(note string, _data *DataTest)

type INodeEndLogger = func(note string, _data *DataTest, _result *ResultTest)

type IOnSuccessFunc = func(_data *DataTest, _result *ResultTest)

type IOnFailFunc = func(_data *DataTest, _result *ResultTest)

type NodeType int64

const (
	NormalNodeType NodeType = iota
	IfNodeType
	ElseNodeType
	ForNodeType
	ParallelNodeType
	ElseIfNodeType
)

type IBasicFlowNode interface {
	SetParentResult(result *ResultTest)
	GetParentResult() *ResultTest
	Run()
	ImplTask() *ResultTest
	SetNext(node IBasicFlowNode)
	GetNext() IBasicFlowNode
	GetNodeType() NodeType
	SetShouldSkip(shouldSkip bool)
	SetNote(note string)
	GetNote() string
	SetBeginLogger(logger INodeBeginLogger)
	GetBeginLogger() INodeBeginLogger
	SetEndLogger(logger INodeEndLogger)
	GetEndLogger() INodeEndLogger
}

type Flow = FlowEngine

func NewFlow() *Flow {
	return NewFlowEngine()
}

//Errors

type ConditionNotFoundError struct{}

func NewConditionNotFoundError() *ConditionNotFoundError {
	return &ConditionNotFoundError{}
}

func (c *ConditionNotFoundError) Error() string {
	return "condition is nil"
}

type PanicHappened struct {
	Msg string
}

func NewPanicHappened(bt string) *PanicHappened {
	return &PanicHappened{Msg: bt}
}

func (c *PanicHappened) Error() string {
	return c.Msg
}

//END Errors

// BasicFlowNode Implementation
type BasicFlowNode struct {
	Functors     []ICallable
	NodeType     NodeType
	Next         IBasicFlowNode
	Data         *DataTest
	ShouldSkip   bool
	parentResult **ResultTest
	BeginLogger  INodeBeginLogger
	EndLogger    INodeEndLogger
	Note         string
}

func NewBasicFlowNode(data *DataTest, parentResult **ResultTest, nodeType NodeType, functors ...ICallable) *BasicFlowNode {
	return &BasicFlowNode{
		Functors:     functors,
		NodeType:     nodeType,
		Data:         data,
		parentResult: parentResult,
	}
}

func (b *BasicFlowNode) SetParentResult(result *ResultTest) {
	*b.parentResult = result
}

func (b *BasicFlowNode) GetParentResult() *ResultTest {
	return *b.parentResult
}

func (b *BasicFlowNode) Run() {
	if b.ShouldSkip || b.GetParentResult().Err != nil || b.GetParentResult().StatusCode != 0 {
		return
	}
	if b.BeginLogger != nil {
		b.BeginLogger(b.Note, b.Data)
	}

	result := b.ImplTask()
	if result != nil {
		b.SetParentResult(result)
	}

	if b.EndLogger != nil {
		b.EndLogger(b.Note, b.Data, b.GetParentResult())
	}
}

func (b *BasicFlowNode) ImplTask() *ResultTest {
	return &ResultTest{
		Err:        nil,
		StatusCode: 0,
		StatusMsg:  "",
	}
}

func (b *BasicFlowNode) GetNext() IBasicFlowNode {
	return b.Next
}

func (b *BasicFlowNode) SetNext(node IBasicFlowNode) {
	b.Next = node
}

func (b *BasicFlowNode) GetNodeType() NodeType {
	return b.NodeType
}

func (b *BasicFlowNode) SetShouldSkip(shouldSkip bool) {
	b.ShouldSkip = shouldSkip
}

func (b *BasicFlowNode) SetNote(note string) {
	b.Note = note
}

func (b *BasicFlowNode) GetNote() string {
	return b.Note
}

func (b *BasicFlowNode) SetBeginLogger(logger INodeBeginLogger) {
	b.BeginLogger = logger
}

func (b *BasicFlowNode) GetBeginLogger() INodeBeginLogger {
	return b.BeginLogger
}

func (b *BasicFlowNode) SetEndLogger(logger INodeEndLogger) {
	b.EndLogger = logger
}

func (b *BasicFlowNode) GetEndLogger() INodeEndLogger {
	return b.EndLogger
}

//END BasicFlowNode

//IfNode Implementation
type IfNode struct {
	*BasicFlowNode
	Condition IBoolFunc
}

func NewIfNode(data *DataTest, parentResult **ResultTest, condition IBoolFunc, functors ...ICallable) *IfNode {
	return &IfNode{
		BasicFlowNode: NewBasicFlowNode(data, parentResult, IfNodeType, functors...),
		Condition:     condition,
	}
}

func (i *IfNode) ImplTask() *ResultTest {
	if i.Condition == nil {
		return &ResultTest{
			Err:        NewConditionNotFoundError(),
			StatusCode: 0,
			StatusMsg:  "",
		}
	}

	if i.Condition(i.Data) {
		for _, functor := range i.Functors {
			result := functor(i.Data)
			if result != nil && (result.Err != nil || result.StatusCode != 0) {
				return result
			}
		}
	}

	current := i.Next
	for current != nil && (current.GetNodeType() == ElseIfNodeType || current.GetNodeType() == ElseNodeType) {
		current.SetShouldSkip(true)
		current = current.GetNext()
	}

	return i.GetParentResult()
}

func (i *IfNode) Run() {
	if i.ShouldSkip || i.GetParentResult().Err != nil || i.GetParentResult().StatusCode != 0 {
		return
	}
	if i.BeginLogger != nil {
		i.BeginLogger(i.Note, i.Data)
	}

	result := i.ImplTask()
	if result != nil {
		i.SetParentResult(result)
	}

	if i.EndLogger != nil {
		i.EndLogger(i.Note, i.Data, i.GetParentResult())
	}
}

//END IfNode

//ElseNode Implementation
type ElseNode struct {
	*BasicFlowNode
}

func NewElseNode(data *DataTest, parentResult **ResultTest, functors ...ICallable) *ElseNode {
	return &ElseNode{NewBasicFlowNode(data, parentResult, ElseNodeType, functors...)}
}

func (e *ElseNode) ImplTask() *ResultTest {
	for _, functor := range e.Functors {
		result := functor(e.Data)
		if result != nil && (result.Err != nil || result.StatusCode != 0) {
			return result
		}
	}
	return e.GetParentResult()
}

func (e *ElseNode) Run() {
	if e.ShouldSkip || e.GetParentResult().Err != nil || e.GetParentResult().StatusCode != 0 {
		return
	}
	if e.BeginLogger != nil {
		e.BeginLogger(e.Note, e.Data)
	}

	result := e.ImplTask()
	if result != nil {
		e.SetParentResult(result)
	}

	if e.EndLogger != nil {
		e.EndLogger(e.Note, e.Data, e.GetParentResult())
	}
}

//END ElseNode

// ElseIfNode Implementation
type ElseIfNode struct {
	*BasicFlowNode
	Condition IBoolFunc
}

func NewElseIfNode(data *DataTest, parentResult **ResultTest, condition IBoolFunc, functors ...ICallable) *ElseIfNode {
	return &ElseIfNode{
		BasicFlowNode: NewBasicFlowNode(data, parentResult, ElseIfNodeType, functors...),
		Condition:     condition,
	}
}

func (e *ElseIfNode) ImplTask() *ResultTest {
	if e.Condition == nil {
		return &ResultTest{
			Err:        NewConditionNotFoundError(),
			StatusCode: 0,
			StatusMsg:  "",
		}
	}

	if e.Condition(e.Data) {
		for _, functor := range e.Functors {
			result := functor(e.Data)
			if result != nil && (result.Err != nil || result.StatusCode != 0) {
				return result
			}
		}
	}

	current := e.Next
	for current != nil && (current.GetNodeType() == ElseIfNodeType || current.GetNodeType() == ElseNodeType) {
		current.SetShouldSkip(true)
		current = current.GetNext()
	}

	return e.GetParentResult()
}

func (e *ElseIfNode) Run() {
	if e.ShouldSkip || e.GetParentResult().Err != nil || e.GetParentResult().StatusCode != 0 {
		return
	}
	if e.BeginLogger != nil {
		e.BeginLogger(e.Note, e.Data)
	}

	result := e.ImplTask()
	if result != nil {
		e.SetParentResult(result)
	}

	if e.EndLogger != nil {
		e.EndLogger(e.Note, e.Data, e.GetParentResult())
	}
}

//END ElseIfNode

//NormalNode Implementation
type NormalNode struct {
	*BasicFlowNode
}

func NewNormalNode(data *DataTest, parentResult **ResultTest, functors ...ICallable) *ElseNode {
	return &ElseNode{NewBasicFlowNode(data, parentResult, NormalNodeType, functors...)}
}

func (n *NormalNode) ImplTask() *ResultTest {
	for _, functor := range n.Functors {
		result := functor(n.Data)
		if result != nil && (result.Err != nil || result.StatusCode != 0) {
			return result
		}
	}
	return n.GetParentResult()
}

func (n *NormalNode) Run() {
	if n.ShouldSkip || n.GetParentResult().Err != nil || n.GetParentResult().StatusCode != 0 {
		return
	}
	if n.BeginLogger != nil {
		n.BeginLogger(n.Note, n.Data)
	}

	result := n.ImplTask()
	if result != nil {
		n.SetParentResult(result)
	}

	if n.EndLogger != nil {
		n.EndLogger(n.Note, n.Data, n.GetParentResult())
	}
}

//END NormalNode

//ForNode Implementation
type ForNode struct {
	*BasicFlowNode
	Times int
}

func NewForNode(times int, data *DataTest, parentResult **ResultTest, functors ...ICallable) *ForNode {
	return &ForNode{
		BasicFlowNode: NewBasicFlowNode(data, parentResult, ForNodeType, functors...),
		Times:         times,
	}
}

func (f *ForNode) ImplTask() *ResultTest {
	for i := 0; i < f.Times; i++ {
		for _, functor := range f.Functors {
			result := functor(f.Data)
			if result != nil && (result.Err != nil || result.StatusCode != 0) {
				return result
			}
		}
	}
	return f.GetParentResult()
}

func (f *ForNode) Run() {
	if f.ShouldSkip || f.GetParentResult().Err != nil || f.GetParentResult().StatusCode != 0 {
		return
	}
	if f.BeginLogger != nil {
		f.BeginLogger(f.Note, f.Data)
	}

	result := f.ImplTask()
	if result != nil {
		f.SetParentResult(result)
	}

	if f.EndLogger != nil {
		f.EndLogger(f.Note, f.Data, f.GetParentResult())
	}
}

//END NormalNode

//ParallelNode Implementation
type ParallelNode struct {
	*BasicFlowNode
	Times int
}

func NewParallelNode(data *DataTest, parentResult **ResultTest, functors ...ICallable) *ParallelNode {
	return &ParallelNode{
		BasicFlowNode: NewBasicFlowNode(data, parentResult, ParallelNodeType, functors...),
	}
}

func (p *ParallelNode) ImplTask() *ResultTest {
	resultChan := make(chan *ResultTest, len(p.Functors))

	wg := sync.WaitGroup{}
	wg.Add(len(p.Functors))

	go func(wg *sync.WaitGroup) {
		wg.Wait()
		close(resultChan)
	}(&wg)

	for _, functor := range p.Functors {
		go func(wg *sync.WaitGroup, f ICallable) {
			defer func() {
				wg.Done()
				if a := recover(); a != nil {
					debug.PrintStack()
					resultChan <- &ResultTest{
						Err:        NewPanicHappened(""),
						StatusCode: 0,
						StatusMsg:  "",
					}
				}
			}()
			result := f(p.Data)
			resultChan <- result
		}(&wg, functor)
	}

	result := p.GetParentResult()
	for item := range resultChan {
		if result != nil && (result.StatusCode != 0 || result.Err != nil) {
			continue
		}
		result = item
	}

	return result
}

func (p *ParallelNode) Run() {
	if p.ShouldSkip || p.GetParentResult().Err != nil || p.GetParentResult().StatusCode != 0 {
		return
	}
	if p.BeginLogger != nil {
		p.BeginLogger(p.Note, p.Data)
	}

	result := p.ImplTask()
	if result != nil {
		p.SetParentResult(result)
	}

	if p.EndLogger != nil {
		p.EndLogger(p.Note, p.Data, p.GetParentResult())
	}
}

//END NormalNode

//FlowEngine Implementation

type FlowEngine struct {
	data          *DataTest
	nodes         []IBasicFlowNode
	result        **ResultTest
	onFailFunc    IOnFailFunc
	onSuccessFunc IOnSuccessFunc
}

func NewFlowEngine() *FlowEngine {
	res := &FlowEngine{
		nodes: make([]IBasicFlowNode, 0, 10),
	}
	tempResult := &ResultTest{
		Err:        nil,
		StatusCode: 0,
		StatusMsg:  "",
	}
	res.data = new(DataTest)
	res.result = &tempResult
	return res
}

func (c *FlowEngine) Prepare(prepareFunc IPrepareFunc, input PrepareTest) *FlowEngine {
	data := prepareFunc(c.data, input)
	c.data = data
	return c
}

func (c *FlowEngine) Do(functors ...ICallable) *FlowEngine {
	node := NewNormalNode(c.data, c.result, functors...)
	if len(c.nodes) != 0 {
		c.nodes[len(c.nodes)-1].SetNext(node)
	}
	c.nodes = append(c.nodes, node)
	return c
}

func (c *FlowEngine) For(times int, functors ...ICallable) *FlowEngine {
	node := NewForNode(times, c.data, c.result, functors...)
	if len(c.nodes) != 0 {
		c.nodes[len(c.nodes)-1].SetNext(node)
	}
	c.nodes = append(c.nodes, node)
	return c
}

func (c *FlowEngine) Parallel(functors ...ICallable) *FlowEngine {
	node := NewParallelNode(c.data, c.result, functors...)
	if len(c.nodes) != 0 {
		c.nodes[len(c.nodes)-1].SetNext(node)
	}
	c.nodes = append(c.nodes, node)
	return c
}

func (c *FlowEngine) If(condition IBoolFunc, functors ...ICallable) *ElseFlowEngine {
	node := NewIfNode(c.data, c.result, condition, functors...)
	if len(c.nodes) != 0 {
		c.nodes[len(c.nodes)-1].SetNext(node)
	}
	c.nodes = append(c.nodes, node)
	return NewElseFlowEngine(&c.data, c, c.result, &c.nodes)
}

func (c *FlowEngine) Wait() *ResultTest {
	for _, node := range c.nodes {
		node.Run()
	}
	if c.onSuccessFunc != nil {
		if (*c.result).Err == nil && (*c.result).StatusCode == 0 {
			c.onSuccessFunc(c.data, *c.result)
		}
	}
	if c.onFailFunc != nil {
		if (*c.result).Err != nil || (*c.result).StatusCode != 0 {
			c.onFailFunc(c.data, *c.result)
		}
	}
	return *c.result
}

func (c *FlowEngine) SetNote(note string) *FlowEngine {
	if len(c.nodes) != 0 {
		c.nodes[len(c.nodes)-1].SetNote(note)
	}
	return c
}

func (c *FlowEngine) SetBeginLogger(logger INodeBeginLogger) *FlowEngine {
	if len(c.nodes) != 0 {
		c.nodes[len(c.nodes)-1].SetBeginLogger(logger)
	}
	return c
}

func (c *FlowEngine) SetEndLogger(logger INodeEndLogger) *FlowEngine {
	if len(c.nodes) != 0 {
		c.nodes[len(c.nodes)-1].SetEndLogger(logger)
	}
	return c
}

func (c *FlowEngine) SetGlobalBeginLogger(logger INodeBeginLogger) *FlowEngine {
	for _, note := range c.nodes {
		if note.GetBeginLogger() == nil {
			note.SetBeginLogger(logger)
		}
	}
	return c
}

func (c *FlowEngine) SetGlobalEndLogger(logger INodeEndLogger) *FlowEngine {
	for _, note := range c.nodes {
		if note.GetEndLogger() == nil {
			note.SetEndLogger(logger)
		}
	}
	return c
}

func (c *FlowEngine) OnFail(functor IOnFailFunc) *FlowEngine {
	c.onFailFunc = functor
	return c
}

func (c *FlowEngine) OnSuccess(functor IOnFailFunc) *FlowEngine {
	c.onSuccessFunc = functor
	return c
}

//END FlowEngine

//ElseFlowEngine implementation

type ElseFlowEngine struct {
	data          **DataTest
	nodes         *[]IBasicFlowNode
	result        **ResultTest
	invoker       *FlowEngine
	onFailFunc    IOnFailFunc
	onSuccessFunc IOnSuccessFunc
}

func NewElseFlowEngine(data **DataTest, invoker *FlowEngine, result **ResultTest, nodes *[]IBasicFlowNode) *ElseFlowEngine {
	res := &ElseFlowEngine{
		data:    data,
		nodes:   nodes,
		result:  result,
		invoker: invoker,
	}
	return res
}

func (e *ElseFlowEngine) Prepare(prepareFunc IPrepareFunc, input PrepareTest) *ElseFlowEngine {
	data := prepareFunc(*e.data, input)
	*e.data = data
	return e
}

func (e *ElseFlowEngine) Do(functors ...ICallable) *FlowEngine {
	node := NewNormalNode(*e.data, e.result, functors...)
	if len(*e.nodes) != 0 {
		(*e.nodes)[len(*e.nodes)-1].SetNext(node)
	}
	*e.nodes = append(*e.nodes, node)
	return e.invoker
}

func (e *ElseFlowEngine) For(times int, functors ...ICallable) *FlowEngine {
	node := NewForNode(times, *e.data, e.result, functors...)
	if len(*e.nodes) != 0 {
		(*e.nodes)[len(*e.nodes)-1].SetNext(node)
	}
	*e.nodes = append(*e.nodes, node)
	return e.invoker
}

func (e *ElseFlowEngine) Parallel(functors ...ICallable) *FlowEngine {
	node := NewParallelNode(*e.data, e.result, functors...)
	if len(*e.nodes) != 0 {
		(*e.nodes)[len(*e.nodes)-1].SetNext(node)
	}
	*e.nodes = append(*e.nodes, node)
	return e.invoker
}

func (e *ElseFlowEngine) If(condition IBoolFunc, functors ...ICallable) *ElseFlowEngine {
	node := NewIfNode(*e.data, e.result, condition, functors...)
	if len(*e.nodes) != 0 {
		(*e.nodes)[len(*e.nodes)-1].SetNext(node)
	}
	*e.nodes = append(*e.nodes, node)
	return e
}

func (e *ElseFlowEngine) ElseIf(condition IBoolFunc, functors ...ICallable) *ElseFlowEngine {
	node := NewElseIfNode(*e.data, e.result, condition, functors...)
	(*e.nodes)[len(*e.nodes)-1].SetNext(node)
	*e.nodes = append(*e.nodes, node)
	return e
}

func (e *ElseFlowEngine) Else(functors ...ICallable) *FlowEngine {
	node := NewElseNode(*e.data, e.result, functors...)
	(*e.nodes)[len(*e.nodes)-1].SetNext(node)
	*e.nodes = append(*e.nodes, node)
	return e.invoker
}

func (e *ElseFlowEngine) Wait() *ResultTest {
	for _, node := range *e.nodes {
		node.Run()
	}
	if e.onSuccessFunc != nil {
		if (*e.result).Err == nil && (*e.result).StatusCode == 0 {
			e.onSuccessFunc(*e.data, *e.result)
		}
	}
	if e.onFailFunc != nil {
		if (*e.result).Err != nil || (*e.result).StatusCode != 0 {
			e.onFailFunc(*e.data, *e.result)
		}
	}
	return *e.result
}

func (e *ElseFlowEngine) SetNote(note string) *ElseFlowEngine {
	if len(*e.nodes) != 0 {
		(*e.nodes)[len(*e.nodes)-1].SetNote(note)
	}
	return e
}

func (e *ElseFlowEngine) SetBeginLogger(logger INodeBeginLogger) *ElseFlowEngine {
	if len(*e.nodes) != 0 {
		(*e.nodes)[len(*e.nodes)-1].SetBeginLogger(logger)
	}
	return e
}

func (e *ElseFlowEngine) SetEndLogger(logger INodeEndLogger) *ElseFlowEngine {
	if len(*e.nodes) != 0 {
		(*e.nodes)[len(*e.nodes)-1].SetEndLogger(logger)
	}
	return e
}

func (e *ElseFlowEngine) SetGlobalBeginLogger(logger INodeBeginLogger) *ElseFlowEngine {
	for _, note := range *e.nodes {
		if note.GetBeginLogger() == nil {
			note.SetBeginLogger(logger)
		}
	}
	return e
}

func (e *ElseFlowEngine) SetGlobalEndLogger(logger INodeEndLogger) *ElseFlowEngine {
	for _, note := range *e.nodes {
		if note.GetEndLogger() == nil {
			note.SetEndLogger(logger)
		}
	}
	return e
}

func (e *ElseFlowEngine) OnFail(functor IOnFailFunc) *ElseFlowEngine {
	e.onFailFunc = functor
	return e
}

func (e *ElseFlowEngine) OnSuccess(functor IOnFailFunc) *ElseFlowEngine {
	e.onSuccessFunc = functor
	return e
}

//END ElseFlowEngine