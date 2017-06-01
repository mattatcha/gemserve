package main

import (
	"io"

	"github.com/samcday/rmarsh"
)

type rubyEncoder struct {
	err error
	gen *rmarsh.Generator
}

func newRubyEncoder(w io.Writer) *rubyEncoder {
	return &rubyEncoder{
		gen: rmarsh.NewGenerator(w),
	}
}

func (r *rubyEncoder) do(fn func() error) error {
	if r.err == nil {
		r.err = fn()
	}
	return r.err
}

func (r *rubyEncoder) StartArray(len int) error {
	return r.do(func() error { return r.gen.StartArray(len) })
}
func (r *rubyEncoder) StartHash(len int) error {
	return r.do(func() error { return r.gen.StartHash(len) })
}
func (r *rubyEncoder) Symbol(sym string) error {
	return r.do(func() error { return r.gen.Symbol(sym) })
}
func (r *rubyEncoder) String(str string) error {
	return r.do(func() error { return r.gen.String(str) })
}
func (r *rubyEncoder) StartIVar(len int) error {
	return r.do(func() error { return r.gen.StartIVar(len) })
}
func (r *rubyEncoder) EndIVar() error {
	return r.do(func() error { return r.gen.EndIVar() })
}
func (r *rubyEncoder) EndArray() error {
	return r.do(func() error { return r.gen.EndArray() })
}
func (r *rubyEncoder) EndHash() error {
	return r.do(func() error { return r.gen.EndHash() })
}

func (r *rubyEncoder) Err() error {
	return r.err
}

//
// g.StartArray(len(vs))
//     g.StartHash(4)
//     g.Symbol("name")
//     g.StartIVar(0)
//     g.String(v.Name)
//     g.EndIVar()
//
//         g.EndArray()
//
//     g.EndHash()
