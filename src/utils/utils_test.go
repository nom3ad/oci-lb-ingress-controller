package utils

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFindJsonPathsForStructs(t *testing.T) {
	src := struct {
		Kind   string
		Count  int
		Name   string
		Weight int
	}{"fruit", 100, "Apple", 500}
	target := struct {
		Kind  string
		Count int
		Name  string
		Color string
	}{"fruit", 200, "Orange", "Yellow"}
	patchMap := FindJsonPatchForStructs(src, target)
	expected := map[string]interface{}{"Color": "Yellow", "Count": 200.0, "Name": "Orange", "Weight": nil}
	assert.True(t, reflect.DeepEqual(patchMap, expected))
	assert.Equal(t, expected, patchMap)

	changeAsString := GetJsonChangesAsString(src, target)
	expectedString := "Color(<nil>=>Yellow)|Count(100=>200)|Name(Apple=>Orange)|Weight(500=><nil>)"
	assert.Equal(t, changeAsString, expectedString)
	assert.Equal(t, GetJsonChangesAsString(src, target, "Count", "Name"), "Color(<nil>=>Yellow)|Weight(500=><nil>)")
}

func FuzzByteAlphaNumericDigest(f *testing.F) {
	f.Add("abcd", 100)
	f.Fuzz(func(t *testing.T, str string, size int) {
		digest := ByteAlphaNumericDigest([]byte(str), size)
		assert.Equal(t, size, len(digest), "Digest has size '%d'", size)
	})
}

func TestByteAlphaNumericDigest(t *testing.T) {
	for i := 0; i < 100; i++ {
		digest := ByteAlphaNumericDigest([]byte("abcd"), i)
		assert.Equal(t, len(digest), i, "Digest was '%s'  len(%d)", digest, len(digest))
	}
}
