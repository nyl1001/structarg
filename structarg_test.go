// Copyright 2019 Yunion
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package structarg

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/nyl1001/pkg/jsonutils"
)

func newParser(d interface{}) (*ArgumentParser, error) {
	p, err := NewArgumentParser(
		d,
		"prog",
		"prog desc",
		"prog epilog",
	)
	return p, err
}

func mustNewParser(t *testing.T, d interface{}) *ArgumentParser {
	p, err := newParser(d)
	if err != nil {
		t.Fatalf("new parser: %v", err)
	}
	return p
}

func TestPositional(t *testing.T) {
	p, err := newParser(
		&struct {
			POS            string
			NONPOSREQUIRED string `positional:"false" required:"true"`
			NONPOS         string `positional:"false"`
		}{},
	)
	if err != nil {
		t.Errorf("error parsing %s", err)
		return
	}
	if len(p.posArgs) != 1 {
		t.Errorf("wrong number of positionals, want 1, got %d", len(p.posArgs))
		return
	} else {
		a := p.posArgs[0]
		if !a.IsPositional() {
			t.Errorf("expecting positional argument, got %s", a)
			return
		}
		if !a.IsRequired() {
			t.Errorf("positional argument should be required, got %s", a)
			return
		}
		if a.String() != "<POS>" {
			t.Errorf("wrong usage %s", a)
			return
		}
	}
	// including help optional argument
	if len(p.optArgs) != 3 {
		t.Errorf("wrong number of optionals, want 3, got %d", len(p.optArgs))
		return
	} else {
		var a Argument
		a = p.optArgs[0]
		if a.IsPositional() {
			t.Errorf("expecting positional argument, got %s", a)
			return
		}
		if a.IsRequired() {
			t.Errorf("expecting non-required optional argument, got %s", a)
			return
		}
		if a.String() != "[--nonpos NONPOS]" {
			t.Errorf("wrong usage %s", a)
			return
		}

		a = p.optArgs[2] // offset by help argument
		if a.IsPositional() {
			t.Errorf("expecting positional argument, got %s", a)
			return
		}
		if !a.IsRequired() {
			t.Errorf("expecting required optional argument, got %s", a)
			return
		}
		if a.String() != "<--nonposrequired NONPOSREQUIRED>" {
			t.Errorf("wrong usage %s", a)
			return
		}
	}
}

func TestNonPositionalOrder(t *testing.T) {
	p := mustNewParser(t,
		&struct {
			Opt0 string
			Opt1 string `required:"true"`
			Opt2 string
			Opt3 string `required:"true"`
			Opt4 string
		}{},
	)
	// including help arg
	if len(p.optArgs) != 6 {
		t.Errorf("num optionals want 6, got %d", len(p.optArgs))
		return
	}
	// make sure that required options come after optional options
	required := false
	for i, arg := range p.optArgs {
		if !arg.IsRequired() {
			if required {
				t.Errorf("bad order at %d", i)
				break
			}
		} else {
			if required == false {
				required = true
			}
		}
	}
}

func TestRequired(t *testing.T) {
	t.Run("optional positional", func(t *testing.T) {
		_, err := newParser(
			&struct {
				POS string `required:"false"`
			}{},
		)
		if err == nil {
			t.Errorf("no error for optional positional")
		}
	})
	t.Run("default positional", func(t *testing.T) {
		_, err := newParser(
			&struct {
				POS string `default:"baddefault"`
			}{},
		)
		if err == nil {
			t.Errorf("no error for positional with default")
		}
	})
	t.Run("required non-positional", func(t *testing.T) {
		p, err := newParser(
			&struct {
				RequiredOpt string `required:"true"`
				Opt         string
			}{},
		)
		if err != nil {
			t.Errorf("errored: %s", err)
			return
		}
		// including help arg
		if len(p.optArgs) != 3 {
			t.Errorf("expecting 3 optArgs, got %d", len(p.optArgs))
			return
		}
		// the required ones should come at last
		a := p.optArgs[2] // offset by help argument
		if !a.IsRequired() {
			t.Errorf("argument %s is optional, want required", a)
			return
		}
		if want := "<--required-opt REQUIRED_OPT>"; a.String() != want {
			t.Errorf("want %s, got %s", want, a.String())
		}
	})
}

func TestNonPositionalRequiredWithDefault(t *testing.T) {
	_, err := newParser(
		&struct {
			Opt int `default:"100" required:"true"`
		}{},
	)
	if err == nil {
		t.Errorf("should error for non-positional argument with default value and required attribute")
	}
}

func TestBoolField(t *testing.T) {
	t.Run("default (no flags)", func(t *testing.T) {
		s := &struct {
			Bool              bool
			BoolP             *bool
			BoolDefaultTrue   bool  `default:"true"`
			BoolPDefaultTrue  *bool `default:"true"`
			BoolDefaultFalse  bool  `default:"false"`
			BoolPDefaultFalse *bool `default:"false"`
		}{}
		p := mustNewParser(t, s)
		args := []string{}
		if err := p.ParseArgs(args, false); err != nil {
			t.Fatalf("ParseArgs failed: %s", err)
		}
		if !(!s.Bool && s.BoolP == nil &&
			s.BoolDefaultTrue && s.BoolPDefaultTrue != nil && *s.BoolPDefaultTrue &&
			!s.BoolDefaultFalse && s.BoolPDefaultFalse != nil && !*s.BoolPDefaultFalse) {
			t.Errorf("wrong parse result: %#v", s)
		}
	})
	t.Run("--flags", func(t *testing.T) {
		s := &struct {
			Bool                bool
			BoolPtr             *bool
			BoolDefaultTrue     bool  `default:"true"`
			BoolPtrDefaultTrue  *bool `default:"true"`
			BoolDefaultFalse    bool  `default:"false"`
			BoolPtrDefaultFalse *bool `default:"false"`
		}{}
		p := mustNewParser(t, s)
		args := []string{
			"--bool",
			"--bool-p",
			"--bool-default-true",
			"--bool-ptr-default-true",
			"--bool-default-false",
			"--bool-ptr-default-false",
		}
		if err := p.ParseArgs(args, false); err != nil {
			t.Fatalf("ParseArgs failed: %s", err)
		}
		if !(s.Bool && s.BoolPtr != nil && *s.BoolPtr &&
			!s.BoolDefaultTrue && s.BoolPtrDefaultTrue != nil && !*s.BoolPtrDefaultTrue &&
			s.BoolDefaultFalse && s.BoolPtrDefaultFalse != nil && *s.BoolPtrDefaultFalse) {
			t.Errorf("wrong parse result: %#v", s)
		}
	})
	t.Run(".conf", func(t *testing.T) {
		s := &struct {
			BoolDefaultTrue bool `default:"true"`
		}{}
		p := mustNewParser(t, s)
		r := bytes.NewBufferString(`
bool_default_true = False
               `)
		if err := p.parseReader(r); err != nil {
			t.Fatalf("parse reader: %v", err)
		}
		if s.BoolDefaultTrue {
			t.Errorf("bool_default_true should be false, got %v", s.BoolDefaultTrue)
		}
	})

}

func TestChoices(t *testing.T) {
	s := &struct {
		String string `choices:"tcp|udp|http|https"`
	}{}
	p := mustNewParser(t, s)
	args := []string{"--string", ""}
	t.Run("good choices", func(t *testing.T) {
		choices := []string{"tcp", "udp", "http", "https"}
		for _, choice := range choices {
			args[1] = choice
			if err := p.ParseArgs(args, false); err != nil {
				t.Fatalf("ParseArgs failed: %s", err)
			}
			if s.String != choice {
				t.Errorf("wrong parse result: want %q, got %q", choice, s.String)
			}
		}
	})
	t.Run("bad choices", func(t *testing.T) {
		choices := []string{"", "et", "ud", "dp"}
		for _, choice := range choices {
			args[1] = choice
			if err := p.ParseArgs(args, false); err == nil {
				t.Fatalf("ParseArgs should error")
			} else {
				errStr := err.Error()
				if !strings.Contains(errStr, ", accepts ") && !strings.Contains(errStr, ", did you mean ") {
					t.Errorf("should error with expected choices, got %q", errStr)
				}
			}
			if s.String != "" {
				t.Errorf("Struct member should not be set, got %s", s.String)
			}
		}
	})
	t.Run("empty as choice", func(t *testing.T) {
		s := &struct {
			OnlyEmpty *string `choices:""`
			HasEmpty  *string `choices:"hello|"`
		}{}
		p := mustNewParser(t, s)

		t.Run("onlyempty: empty", func(t *testing.T) {
			if err := p.ParseArgs([]string{"--only-empty", ""}, false); err != nil {
				t.Errorf("got err: %v", err)
			} else if s.OnlyEmpty == nil {
				t.Errorf("got nil")
			}
		})

		s.OnlyEmpty = nil
		t.Run("onlyempty: notempty", func(t *testing.T) {
			if err := p.ParseArgs([]string{"--only-empty", "notempty"}, false); err == nil {
				t.Errorf("no err")
			}
			if s.OnlyEmpty != nil {
				t.Errorf("got nil")
			}
		})

		t.Run("hasempty: good", func(t *testing.T) {
			want := "hello"
			if err := p.ParseArgs([]string{"--has-empty", want}, false); err != nil {
				t.Errorf("got err: %v", err)
			} else if s.HasEmpty == nil {
				t.Errorf("got nil")
			} else if *s.HasEmpty != want {
				t.Errorf("got %q, want %q", *s.HasEmpty, want)
			}
		})

		s.HasEmpty = nil
		t.Run("hasempty: empty", func(t *testing.T) {
			want := ""
			if err := p.ParseArgs([]string{"--has-empty", want}, false); err != nil {
				t.Errorf("got err: %v", err)
			} else if s.HasEmpty == nil {
				t.Errorf("got nil")
			} else if *s.HasEmpty != want {
				t.Errorf("got %q, want %q", *s.HasEmpty, want)
			}
		})

		s.HasEmpty = nil
		t.Run("hasempty: bad", func(t *testing.T) {
			arg := "world"
			if err := p.ParseArgs([]string{"--has-empty", arg}, false); err == nil {
				t.Errorf("no err")
			} else if s.HasEmpty != nil {
				t.Errorf("got %q, want nil", *s.HasEmpty)
			}
		})

	})
}

func TestArgValue(t *testing.T) {
	s := &struct {
		String string
	}{}
	p := mustNewParser(t, s)
	args := []string{"--string", ""}
	cases := []struct {
		name  string
		value string
	}{
		{
			name:  "with space",
			value: `Hello world`,
		},
		{
			name:  "with single quote",
			value: `'Hello 'world'`,
		},
		{
			name:  "with double quote",
			value: `"Hello "world"`,
		},
		{
			name:  "with newline and tab",
			value: `Hello\n\tworld\n`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			args[1] = c.value
			err := p.ParseArgs(args, false)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if s.String != c.value {
				t.Errorf("want %s, got %s", c.value, s.String)
			}
		})
	}
}

func TestIgnoreUnexported(t *testing.T) {
	s := &struct {
		unexported string
	}{}
	p := mustNewParser(t, s)
	args := []string{"--string", ""}
	p.ParseArgs(args, true)
}

func TestStructMember(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		type L struct {
			NonPos string
			POS    string
		}
		s := &struct {
			L
			M struct {
				NonPos string
				POS    string
			}
			N struct {
				NonPos string
				POS    string
			}
		}{}
		p := mustNewParser(t, s)
		args := []string{
			"--non-pos", "l-non-pos",
			"--m-non-pos", "m-non-pos",
			"--n-non-pos", "n-non-pos",
			"L_POS",
			"M_POS",
			"N_POS",
		}
		if err := p.ParseArgs(args, true); err != nil {
			t.Fatalf("ParseArgs failed: %s", err)
		}
		if s.L.NonPos != "l-non-pos" || s.L.POS != "L_POS" ||
			s.M.NonPos != "m-non-pos" || s.M.POS != "M_POS" ||
			s.N.NonPos != "n-non-pos" || s.N.POS != "N_POS" {
			t.Errorf("something does not match\npassed: %#v\ngot: %#v", args, s)
		}
	})
	t.Run("name duplicate (embedded vs. non-embedded)", func(t *testing.T) {
		type L struct {
			MNonPos string `token:"m-non-pos"`
			POS     string
		}
		s := &struct {
			L
			M struct {
				NonPos string
				POS    string
			}
		}{}
		_, err := newParser(s)
		if err == nil {
			t.Fatalf("expecting error")
		}
	})
	t.Run("name duplicate (across embedded)", func(t *testing.T) {
		type L struct {
			NonPos string
			POS    string
		}
		type M struct {
			NonPos string
			POS    string
		}
		s := &struct {
			L
			M
		}{}
		_, err := newParser(s)
		if err == nil {
			t.Fatalf("expecting error")
		}
	})
}

func TestArgumentParser_parseJSONDict(t *testing.T) {
	toJSONDict := func(input map[string]interface{}) *jsonutils.JSONDict {
		return jsonutils.Marshal(input).(*jsonutils.JSONDict)
	}

	tests := []struct {
		name string
		//parser  *ArgumentParser
		inputTarget interface{}
		args        *jsonutils.JSONDict
		wantTarget  interface{}
		wantErr     bool
	}{
		{
			name: "parse json dict",
			inputTarget: &struct {
				Name     string
				Number   int
				Networks []string
				Quoted   string
				Quotes   []string
			}{},
			args: toJSONDict(map[string]interface{}{
				"name":     "args",
				"number":   1,
				"networks": []string{"net1", "net2"},
				"quoted":   `"1"`,
				"quotes":   []string{`"1"`, `"2"`},
			}),
			wantTarget: &struct {
				Name     string
				Number   int
				Networks []string
				Quoted   string
				Quotes   []string
			}{
				Name:     "args",
				Number:   1,
				Networks: []string{"net1", "net2"},
				Quoted:   `"1"`,
				Quotes:   []string{`"1"`, `"2"`},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser, err := newParser(tt.inputTarget)
			if err != nil {
				t.Errorf("newParser: %v", err)
				return
			}
			if err := parser.parseJSONDict(tt.args); (err != nil) != tt.wantErr {
				t.Errorf("ArgumentParser.parseJSONDict() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(tt.inputTarget, tt.wantTarget) {
				t.Errorf("inputTarget = %#v, wantTarget = %#v", tt.inputTarget, tt.wantTarget)
				return
			}
			t.Logf("Parser target: %#v", tt.inputTarget)
		})
	}
}

func Test_keyToToken(t *testing.T) {
	type args struct {
		key string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "dns_domain_a",
			args: args{"dns_domain_a"},
			want: "dns-domain-a",
		},
		{
			name: "test-case_1",
			args: args{"test-case_1"},
			want: "test-case-1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := keyToToken(tt.args.key); got != tt.want {
				t.Errorf("keyToToken() = %v, want %v", got, tt.want)
			}
		})
	}
}
