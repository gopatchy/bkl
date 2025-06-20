package bkl

import (
	"fmt"
)

func repeatDoc(doc *Document, ec *EvalContext) ([]*Document, []*EvalContext, error) {
	switch obj := doc.Data.(type) {
	case map[string]any:
		return repeatDocMap(doc, ec, obj)

	case []any:
		return repeatDocList(doc, ec, obj)

	default:
		return []*Document{doc}, []*EvalContext{ec}, nil
	}
}

func repeatDocMap(doc *Document, ec *EvalContext, data map[string]any) ([]*Document, []*EvalContext, error) {
	if found, v, data := popMapValue(data, "$repeat"); found {
		doc.Data = data
		return repeatDocGen(doc, ec, v)
	}

	return []*Document{doc}, []*EvalContext{ec}, nil
}

func repeatDocList(doc *Document, ec *EvalContext, data []any) ([]*Document, []*EvalContext, error) {
	v, data2, err := popListMapValue(data, "$repeat")
	if err != nil {
		return nil, nil, err
	}

	if v != nil {
		doc.Data = data2
		return repeatDocGen(doc, ec, v)
	}

	return []*Document{doc}, []*EvalContext{ec}, nil
}

func repeatDocGen(doc *Document, ec *EvalContext, v any) ([]*Document, []*EvalContext, error) {
	switch v2 := v.(type) {
	case int:
		return repeatDocGenFromInt(doc, ec, "$repeat", v2)

	case map[string]any:
		return repeatDocGenFromMap(doc, ec, v2)

	default:
		return nil, nil, fmt.Errorf("$repeat: %T (%w)", v, ErrInvalidType)
	}
}

func repeatDocGenFromInt(doc *Document, ec *EvalContext, name string, count int) ([]*Document, []*EvalContext, error) {
	docs := []*Document{}
	ecs := []*EvalContext{}

	for i := 0; i < count; i++ {
		doc2, err := doc.Clone(fmt.Sprintf("%s=%d", name, i))
		if err != nil {
			return nil, nil, err
		}

		ec2 := ec.Clone()
		ec2.Vars[name] = i

		docs = append(docs, doc2)
		ecs = append(ecs, ec2)
	}

	return docs, ecs, nil
}

func repeatDocGenFromMap(doc *Document, ec *EvalContext, rs map[string]any) ([]*Document, []*EvalContext, error) {
	if repeatIsRangeParamsMap(rs) {
		return repeatDocGenFromRangeParams(doc, ec, rs)
	}

	ec = ec.Clone()

	for k, v := range rs {
		ec.Vars[fmt.Sprintf("$repeat.%s", k)] = v
	}

	docs := []*Document{doc}
	ecs := []*EvalContext{ec}

	for name, count := range sortedMap(rs) {
		count2, ok := count.(int)
		if !ok {
			return nil, nil, fmt.Errorf("%T (%w)", count, ErrInvalidRepeat)
		}

		tmpDocs := []*Document{}
		tmpECs := []*EvalContext{}

		for i, d := range docs {
			ds, es, err := repeatDocGenFromInt(d, ecs[i], fmt.Sprintf("$repeat:%s", name), count2)
			if err != nil {
				return nil, nil, err
			}

			tmpDocs = append(tmpDocs, ds...)
			tmpECs = append(tmpECs, es...)
		}

		docs = tmpDocs
		ecs = tmpECs
	}

	return docs, ecs, nil
}

func repeatIsRangeParamsMap(rs map[string]any) bool {
	for k := range rs {
		switch k {
		case "$first", "$last", "$count", "$step":
			return true
		}
	}
	return false
}

func repeatDocGenFromRangeParams(doc *Document, ec *EvalContext, rs map[string]any) ([]*Document, []*EvalContext, error) {
	first, hasFirst := getMapIntValue(rs, "$first")
	last, hasLast := getMapIntValue(rs, "$last")
	count, hasCount := getMapIntValue(rs, "$count")
	step, hasStep := getMapIntValue(rs, "$step")

	if !hasStep {
		step = 1
	}

	if step == 0 {
		return nil, nil, fmt.Errorf("$step cannot be 0 (%w)", ErrInvalidRepeat)
	}

	if hasCount && count <= 0 {
		return nil, nil, fmt.Errorf("$count=%d must be positive (%w)", count, ErrInvalidRepeat)
	}

	if hasFirst && hasLast && hasCount {
		return nil, nil, fmt.Errorf("cannot specify all of $first, $last, and $count (%w)", ErrInvalidRepeat)
	} else if hasFirst && hasLast {
		if (last-first)%step != 0 {
			return nil, nil, fmt.Errorf("$last=%d - $first=%d must be divisible by $step=%d (%w)", last, first, step, ErrInvalidRepeat)
		}
	} else if hasFirst && hasCount {
		last = first + (count-1)*step
	} else if hasLast && hasCount {
		first = last - (count-1)*step
	} else {
		return nil, nil, fmt.Errorf("must specify exactly 2 of $first, $last, $count (%w)", ErrInvalidRepeat)
	}

	docs := []*Document{}
	ecs := []*EvalContext{}

	for value := first; value != last+step; value += step {
		doc2, err := doc.Clone(fmt.Sprintf("$repeat=%d", value))
		if err != nil {
			return nil, nil, err
		}

		ec2 := ec.Clone()
		ec2.Vars["$repeat"] = value

		docs = append(docs, doc2)
		ecs = append(ecs, ec2)
	}

	return docs, ecs, nil
}
