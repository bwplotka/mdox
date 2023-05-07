// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

// Taken from Thanos project.
//
// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package yamlgen

import (
	"errors"
	"fmt"
	"io"
	"reflect"

	"github.com/fatih/structtag"
	"gopkg.in/yaml.v3"
)

func Generate(obj interface{}, w io.Writer) error {
	// We forbid omitempty option. This is for simplification for doc generation.
	if err := checkForOmitEmptyTagOption(obj); err != nil {
		return fmt.Errorf("invalid type: %w", err)
	}
	return yaml.NewEncoder(w).Encode(obj)
}

func checkForOmitEmptyTagOption(obj interface{}) error {
	return checkForOmitEmptyTagOptionRec(reflect.ValueOf(obj))
}

func checkForOmitEmptyTagOptionRec(v reflect.Value) error {
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			tags, err := structtag.Parse(string(v.Type().Field(i).Tag))
			if err != nil {
				return fmt.Errorf("%s: failed to parse tag %q: err: %w", v.Type().Field(i).Name, v.Type().Field(i).Tag, err)
			}

			tag, err := tags.Get("yaml")
			if err != nil {
				return fmt.Errorf("%s: failed to get tag %q: %w", v.Type().Field(i).Name, v.Type().Field(i).Tag, err)
			}

			for _, opts := range tag.Options {
				if opts == "omitempty" {
					return fmt.Errorf("omitempty is forbidden for config, but spotted on field '%s'", v.Type().Field(i).Name)
				}
			}

			if err := checkForOmitEmptyTagOptionRec(v.Field(i)); err != nil {
				return fmt.Errorf("%s: %w", v.Type().Field(i).Name, err)
			}
		}

	case reflect.Ptr:
		return errors.New("nil pointers are not allowed in configuration")

	case reflect.Interface:
		return checkForOmitEmptyTagOptionRec(v.Elem())
	}

	return nil
}
