package metrics

func ErrToStatusLabel(err error) string {
	if err != nil {
		return "error"
	}
	return "success"
}

func ErrToStatusLabels(err error) []string {
	return []string{
		ErrToStatusLabel(err),
	}
}
