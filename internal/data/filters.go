package data

import "github.com/VJ-2303/code-runner/internal/validator"

type Filters struct {
	Language string
}

func ValidateFilters(v *validator.Validator, f Filters) {
	if f.Language != "" {
		v.Check(validator.PermittedValue(f.Language, "ruby", "python", "javascript"), "language", "must be either ruby, python or javascript")
	}
}
