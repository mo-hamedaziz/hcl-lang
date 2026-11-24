// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package reference

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/hcl-lang/lang"
	"github.com/hashicorp/hcl-lang/schema"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty-debug/ctydebug"
	"github.com/zclconf/go-cty/cty"
)

func TestTarget_Address(t *testing.T) {
	testCases := []struct {
		name            string
		pos             hcl.Pos
		activeSelfRefs  bool
		target          Target
		expectedAddress lang.Address
	}{
		{
			"absolute address and no local address",
			hcl.InitialPos,
			false,
			Target{
				Addr: lang.Address{
					lang.RootStep{Name: "aws_instance"},
					lang.AttrStep{Name: "instance_size"},
				},
			},
			lang.Address{
				lang.RootStep{Name: "aws_instance"},
				lang.AttrStep{Name: "instance_size"},
			},
		},
		{
			"local address and no absolute address",
			hcl.InitialPos,
			false,
			Target{
				LocalAddr: lang.Address{
					lang.RootStep{Name: "count"},
					lang.AttrStep{Name: "index"},
				},
			},
			lang.Address{
				lang.RootStep{Name: "count"},
				lang.AttrStep{Name: "index"},
			},
		},
		{
			"self address with active self and matching range",
			hcl.Pos{Line: 2, Column: 2, Byte: 2},
			true,
			Target{
				Addr: lang.Address{
					lang.RootStep{Name: "aws_instance"},
					lang.AttrStep{Name: "instance_size"},
				},
				LocalAddr: lang.Address{
					lang.RootStep{Name: "self"},
					lang.AttrStep{Name: "instance_size"},
				},
				TargetableFromRangePtr: &hcl.Range{
					Filename: "test.tf",
					Start:    hcl.Pos{Line: 1, Column: 1, Byte: 0},
					End:      hcl.Pos{Line: 3, Column: 1, Byte: 10},
				},
			},
			lang.Address{
				lang.RootStep{Name: "self"},
				lang.AttrStep{Name: "instance_size"},
			},
		},
		{
			"self address without active self but matching range",
			hcl.Pos{Line: 2, Column: 2, Byte: 2},
			false,
			Target{
				Addr: lang.Address{
					lang.RootStep{Name: "aws_instance"},
					lang.AttrStep{Name: "instance_size"},
				},
				LocalAddr: lang.Address{
					lang.RootStep{Name: "self"},
					lang.AttrStep{Name: "instance_size"},
				},
				TargetableFromRangePtr: &hcl.Range{
					Filename: "test.tf",
					Start:    hcl.Pos{Line: 1, Column: 1, Byte: 0},
					End:      hcl.Pos{Line: 3, Column: 1, Byte: 10},
				},
			},
			lang.Address{
				lang.RootStep{Name: "aws_instance"},
				lang.AttrStep{Name: "instance_size"},
			},
		},
		{
			"self address with active self but no matching range",
			hcl.Pos{Line: 5, Column: 2, Byte: 15},
			true,
			Target{
				Addr: lang.Address{
					lang.RootStep{Name: "aws_instance"},
					lang.AttrStep{Name: "instance_size"},
				},
				LocalAddr: lang.Address{
					lang.RootStep{Name: "self"},
					lang.AttrStep{Name: "instance_size"},
				},
				TargetableFromRangePtr: &hcl.Range{
					Filename: "test.tf",
					Start:    hcl.Pos{Line: 1, Column: 1, Byte: 0},
					End:      hcl.Pos{Line: 3, Column: 1, Byte: 10},
				},
			},
			lang.Address{
				lang.RootStep{Name: "aws_instance"},
				lang.AttrStep{Name: "instance_size"},
			},
		},
		{
			"self address with active self and missing targetable",
			hcl.Pos{Line: 5, Column: 2, Byte: 15},
			true,
			Target{
				Addr: lang.Address{
					lang.RootStep{Name: "aws_instance"},
					lang.AttrStep{Name: "instance_size"},
				},
				LocalAddr: lang.Address{
					lang.RootStep{Name: "self"},
					lang.AttrStep{Name: "instance_size"},
				},
			},
			lang.Address{
				lang.RootStep{Name: "aws_instance"},
				lang.AttrStep{Name: "instance_size"},
			},
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("%d-%s", i, tc.name), func(t *testing.T) {
			ctx := context.Background()

			if tc.activeSelfRefs {
				ctx = schema.WithActiveSelfRefs(ctx)
			}

			address := tc.target.Address(ctx, tc.pos)
			if diff := cmp.Diff(tc.expectedAddress, address, ctydebug.CmpOptions); diff != "" {
				t.Fatalf("mismatch of address: %s", diff)
			}
		})
	}
}

func TestTarget_Matches_nestedType(t *testing.T) {
	varAddr := func(name string, steps ...lang.AddressStep) lang.Address {
		return append(lang.Address{
			lang.RootStep{Name: "var"},
			lang.AttrStep{Name: name},
		}, steps...)
	}
	idx := func(key cty.Value) lang.IndexStep {
		return lang.IndexStep{Key: key}
	}
	attr := func(name string) lang.AttrStep {
		return lang.AttrStep{Name: name}
	}

	testCases := []struct {
		name          string
		target        Target
		origin        MatchableOrigin
		expectedMatch bool
	}{
		{
			"map index matching element type",
			Target{Addr: varAddr("plain"), Type: cty.Map(cty.List(cty.String))},
			LocalOrigin{
				Addr:        varAddr("plain", idx(cty.StringVal("k"))),
				Constraints: OriginConstraints{{OfType: cty.List(cty.String)}},
			},
			true,
		},
		{
			"map index with dynamic constraint",
			Target{Addr: varAddr("plain"), Type: cty.Map(cty.List(cty.String))},
			LocalOrigin{
				Addr:        varAddr("plain", idx(cty.StringVal("k"))),
				Constraints: OriginConstraints{{OfType: cty.DynamicPseudoType}},
			},
			true,
		},
		{
			"map index with incompatible constraint",
			Target{Addr: varAddr("plain"), Type: cty.Map(cty.List(cty.String))},
			LocalOrigin{
				Addr:        varAddr("plain", idx(cty.StringVal("k"))),
				Constraints: OriginConstraints{{OfType: cty.Bool}},
			},
			false,
		},
		{
			"map index with mismatching scope",
			Target{Addr: varAddr("plain"), Type: cty.Map(cty.String), ScopeId: lang.ScopeId("variable")},
			LocalOrigin{
				Addr:        varAddr("plain", idx(cty.StringVal("k"))),
				Constraints: OriginConstraints{{OfScopeId: lang.ScopeId("local"), OfType: cty.String}},
			},
			false,
		},
		{
			"map attribute access",
			Target{Addr: varAddr("plain"), Type: cty.Map(cty.String)},
			LocalOrigin{
				Addr:        varAddr("plain", attr("k")),
				Constraints: OriginConstraints{{OfType: cty.String}},
			},
			true,
		},
		{
			"map with numeric index",
			Target{Addr: varAddr("plain"), Type: cty.Map(cty.String)},
			LocalOrigin{
				Addr:        varAddr("plain", idx(cty.NumberIntVal(0))),
				Constraints: OriginConstraints{{OfType: cty.String}},
			},
			false,
		},
		{
			"multiple nested steps",
			Target{Addr: varAddr("plain"), Type: cty.Map(cty.List(cty.String))},
			LocalOrigin{
				Addr:        varAddr("plain", idx(cty.StringVal("k")), idx(cty.NumberIntVal(0))),
				Constraints: OriginConstraints{{OfType: cty.String}},
			},
			true,
		},
		{
			"map of any",
			Target{Addr: varAddr("plain"), Type: cty.Map(cty.DynamicPseudoType)},
			LocalOrigin{
				Addr:        varAddr("plain", idx(cty.StringVal("k")), attr("foo")),
				Constraints: OriginConstraints{{OfType: cty.String}},
			},
			true,
		},
		{
			"list index",
			Target{Addr: varAddr("list"), Type: cty.List(cty.String)},
			LocalOrigin{
				Addr:        varAddr("list", idx(cty.NumberIntVal(0))),
				Constraints: OriginConstraints{{OfType: cty.String}},
			},
			true,
		},
		{
			"list with string index",
			Target{Addr: varAddr("list"), Type: cty.List(cty.String)},
			LocalOrigin{
				Addr:        varAddr("list", idx(cty.StringVal("k"))),
				Constraints: OriginConstraints{{OfType: cty.String}},
			},
			false,
		},
		{
			"list attribute access",
			Target{Addr: varAddr("list"), Type: cty.List(cty.String)},
			LocalOrigin{
				Addr:        varAddr("list", attr("foo")),
				Constraints: OriginConstraints{{OfType: cty.String}},
			},
			false,
		},
		{
			"set index",
			Target{Addr: varAddr("set"), Type: cty.Set(cty.String)},
			LocalOrigin{
				Addr:        varAddr("set", idx(cty.NumberIntVal(0))),
				Constraints: OriginConstraints{{OfType: cty.String}},
			},
			false,
		},
		{
			"tuple index",
			Target{Addr: varAddr("tuple"), Type: cty.Tuple([]cty.Type{cty.String, cty.Bool})},
			LocalOrigin{
				Addr:        varAddr("tuple", idx(cty.NumberIntVal(1))),
				Constraints: OriginConstraints{{OfType: cty.Bool}},
			},
			true,
		},
		{
			"tuple index out of range",
			Target{Addr: varAddr("tuple"), Type: cty.Tuple([]cty.Type{cty.String, cty.Bool})},
			LocalOrigin{
				Addr:        varAddr("tuple", idx(cty.NumberIntVal(2))),
				Constraints: OriginConstraints{{OfType: cty.DynamicPseudoType}},
			},
			false,
		},
		{
			"object attribute",
			Target{Addr: varAddr("obj"), Type: cty.Object(map[string]cty.Type{"foo": cty.String})},
			LocalOrigin{
				Addr:        varAddr("obj", attr("foo")),
				Constraints: OriginConstraints{{OfType: cty.String}},
			},
			true,
		},
		{
			"object index by attribute name",
			Target{Addr: varAddr("obj"), Type: cty.Object(map[string]cty.Type{"foo": cty.String})},
			LocalOrigin{
				Addr:        varAddr("obj", idx(cty.StringVal("foo"))),
				Constraints: OriginConstraints{{OfType: cty.String}},
			},
			true,
		},
		{
			"object unknown attribute",
			Target{Addr: varAddr("obj"), Type: cty.Object(map[string]cty.Type{"foo": cty.String})},
			LocalOrigin{
				Addr:        varAddr("obj", attr("bar")),
				Constraints: OriginConstraints{{OfType: cty.DynamicPseudoType}},
			},
			false,
		},
		{
			"primitive type",
			Target{Addr: varAddr("str"), Type: cty.String},
			LocalOrigin{
				Addr:        varAddr("str", idx(cty.NumberIntVal(0))),
				Constraints: OriginConstraints{{OfType: cty.DynamicPseudoType}},
			},
			false,
		},
		{
			"different target address",
			Target{Addr: varAddr("other"), Type: cty.Map(cty.String)},
			LocalOrigin{
				Addr:        varAddr("plain", idx(cty.StringVal("k"))),
				Constraints: OriginConstraints{{OfType: cty.String}},
			},
			false,
		},
		{
			"local address",
			Target{
				LocalAddr: lang.Address{lang.RootStep{Name: "self"}, attr("tags")},
				Type:      cty.Map(cty.String),
			},
			LocalOrigin{
				Addr: lang.Address{
					lang.RootStep{Name: "self"},
					attr("tags"),
					idx(cty.StringVal("Name")),
				},
				Constraints: OriginConstraints{{OfType: cty.String}},
			},
			true,
		},
		{
			"target with nested targets",
			Target{
				Addr: varAddr("obj"),
				Type: cty.Object(map[string]cty.Type{"foo": cty.String}),
				NestedTargets: Targets{
					{Addr: varAddr("obj", attr("foo")), Type: cty.String},
				},
			},
			LocalOrigin{
				Addr:        varAddr("obj", attr("foo")),
				Constraints: OriginConstraints{{OfType: cty.String}},
			},
			false,
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("%d-%s", i, tc.name), func(t *testing.T) {
			match := tc.target.Matches(tc.origin)
			if match != tc.expectedMatch {
				t.Fatalf("expected match: %t, given: %t", tc.expectedMatch, match)
			}
		})
	}
}

func TestTargets_Match_nestedType_noDuplicates(t *testing.T) {
	objType := cty.Object(map[string]cty.Type{"foo": cty.String})
	nested := Target{
		Addr: lang.Address{
			lang.RootStep{Name: "var"},
			lang.AttrStep{Name: "obj"},
			lang.AttrStep{Name: "foo"},
		},
		Type: cty.String,
	}
	targets := Targets{
		{
			Addr: lang.Address{
				lang.RootStep{Name: "var"},
				lang.AttrStep{Name: "obj"},
			},
			Type:          objType,
			NestedTargets: Targets{nested},
		},
	}
	origin := LocalOrigin{
		Addr:        nested.Addr,
		Constraints: OriginConstraints{{OfType: cty.String}},
	}

	matched, ok := targets.Match(origin)
	if !ok {
		t.Fatal("expected a match")
	}
	if diff := cmp.Diff(Targets{nested}, matched, ctydebug.CmpOptions); diff != "" {
		t.Fatalf("unexpected targets: %s", diff)
	}
}
