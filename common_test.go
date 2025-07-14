package bkl_test

func getFormat(languages [][]any) *string {
	if len(languages) > 0 && len(languages[0]) > 1 {
		if format, ok := languages[0][1].(string); ok {
			return &format
		}
	}
	return nil
}
