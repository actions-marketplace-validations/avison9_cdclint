package engine

import "regexp"

func compileAnchored(p string) (func(string) bool, error) {
	re, err := regexp.Compile("(?i)^(?:" + p + ")$")
	if err != nil {
		return nil, err
	}
	return re.MatchString, nil
}
