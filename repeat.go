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
	contexts, err := repeatGenerateContexts(ec, v)
	if err != nil {
		return nil, nil, err
	}
	
	docs := make([]*Document, len(contexts))
	ecs := make([]*EvalContext, len(contexts))
	
	for i, ctx := range contexts {
		doc2, err := doc.Clone(fmt.Sprintf("repeat-%d", i))
		if err != nil {
			return nil, nil, err
		}
		docs[i] = doc2
		ecs[i] = ctx
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





func repeatGetRangeParamValues(rs map[string]any) ([]any, error) {
	first, hasFirst := getMapIntValue(rs, "$first")
	last, hasLast := getMapIntValue(rs, "$last")
	count, hasCount := getMapIntValue(rs, "$count")
	step, hasStep := getMapIntValue(rs, "$step")

	if !hasStep {
		step = 1
	}

	if step == 0 {
		return nil, fmt.Errorf("$step cannot be 0 (%w)", ErrInvalidRepeat)
	}

	if hasCount && count <= 0 {
		return nil, fmt.Errorf("$count=%d must be positive (%w)", count, ErrInvalidRepeat)
	}

	if hasFirst && hasLast && hasCount {
		return nil, fmt.Errorf("cannot specify all of $first, $last, and $count (%w)", ErrInvalidRepeat)
	} else if hasFirst && hasLast {
		if (last-first)%step != 0 {
			return nil, fmt.Errorf("$last=%d - $first=%d must be divisible by $step=%d (%w)", last, first, step, ErrInvalidRepeat)
		}
	} else if hasFirst && hasCount {
		last = first + (count-1)*step
	} else if hasLast && hasCount {
		first = last - (count-1)*step
	} else {
		return nil, fmt.Errorf("must specify exactly 2 of $first, $last, $count (%w)", ErrInvalidRepeat)
	}

	var values []any
	for value := first; value != last+step; value += step {
		values = append(values, value)
	}

	return values, nil
}

func repeatGenerateContexts(ec *EvalContext, r any) ([]*EvalContext, error) {
	switch r2 := r.(type) {
	case int:
		contexts := make([]*EvalContext, r2)
		for i := 0; i < r2; i++ {
			ctx := ec.Clone()
			ctx.Vars["$repeat"] = i
			contexts[i] = ctx
		}
		return contexts, nil
		
	case []any:
		contexts := make([]*EvalContext, len(r2))
		for i, value := range r2 {
			ctx := ec.Clone()
			ctx.Vars["$repeat"] = value
			contexts[i] = ctx
		}
		return contexts, nil
		
	case map[string]any:
		if repeatIsRangeParamsMap(r2) {
			values, err := repeatGetRangeParamValues(r2)
			if err != nil {
				return nil, err
			}
			contexts := make([]*EvalContext, len(values))
			for i, value := range values {
				ctx := ec.Clone()
				ctx.Vars["$repeat"] = value
				contexts[i] = ctx
			}
			return contexts, nil
		}
		
		return repeatGenerateContextsFromMap(ec, r2)
		
	default:
		return nil, fmt.Errorf("$repeat: %T (%w)", r, ErrInvalidType)
	}
}

func repeatGenerateContextsFromMap(ec *EvalContext, rs map[string]any) ([]*EvalContext, error) {
	ec = ec.Clone()
	for k, v := range rs {
		ec.Vars[fmt.Sprintf("$repeat.%s", k)] = v
	}
	
	contexts := []*EvalContext{ec}
	
	for name, value := range sortedMap(rs) {
		var newContexts []*EvalContext
		var values []any
		var err error
		
		switch v := value.(type) {
		case int:
			values = make([]any, v)
			for i := 0; i < v; i++ {
				values[i] = i
			}
			
		case []any:
			values = v
			
		case map[string]any:
			if repeatIsRangeParamsMap(v) {
				values, err = repeatGetRangeParamValues(v)
				if err != nil {
					return nil, err
				}
			} else {
				return nil, fmt.Errorf("%s: map must contain range parameters ($first, $last, $count, $step) (%w)", name, ErrInvalidRepeat)
			}
			
		default:
			return nil, fmt.Errorf("%s: %T (%w)", name, value, ErrInvalidRepeat)
		}
		
		for _, ctx := range contexts {
			for _, item := range values {
				newCtx := ctx.Clone()
				newCtx.Vars[fmt.Sprintf("$repeat:%s", name)] = item
				newContexts = append(newContexts, newCtx)
			}
		}
		
		contexts = newContexts
	}
	
	return contexts, nil
}