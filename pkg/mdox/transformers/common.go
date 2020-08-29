// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package transformers

type RemoveFrontMatter struct{}

func (RemoveFrontMatter) TransformFrontMatter(_ string, frontMatter map[string]interface{}) ([]byte, error) {
	for k := range frontMatter {
		delete(frontMatter, k)
	}
	return nil, nil
}
