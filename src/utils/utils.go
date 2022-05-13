package utils

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/fatih/structs"
	"k8s.io/apimachinery/pkg/util/sets"
)

func NullOrDeepEqual(x, y interface{}) bool {
	z := reflect.DeepEqual(x, y)
	if x == nil && y == nil {
		return true
	}
	return z
}

// PtrToString returns a pointer to the provided string
func PtrToString(value string) *string {
	return &value
}

//PtrToBool returns a pointer to the provided bool
func PtrToBool(value bool) *bool {
	return &value
}

// Int returns a pointer to the provided int
func PtrToInt(value int) *int {
	return &value
}

// Int64 returns a pointer to the provided int64
func PtrToInt64(value int64) *int64 {
	return &value
}

// Uint returns a pointer to the provided uint
func PtrToUint(value uint) *uint {
	return &value
}

//Float32 returns a pointer to the provided float32
func PtrToFloat32(value float32) *float32 {
	return &value
}

//Float64 returns a pointer to the provided float64
func PtrToFloat64(value float64) *float64 {
	return &value
}

func ObjectHash(obj interface{}, length int) string {
	var buf bytes.Buffer
	_ = gob.NewEncoder(&buf).Encode(obj)
	return ByteAlphaNumericDigest(buf.Bytes(), length)
}

// ByteAlphaNumericDigest returns digest of given string (22 char length)
func ByteAlphaNumericDigest(bytes []byte, length int) string {
	gen := func(bytes []byte) string {
		hash := md5.Sum(bytes)
		encoded := base64.StdEncoding.EncodeToString(hash[:])
		encoded = strings.TrimRight(encoded, "=")
		encoded = strings.ReplaceAll(encoded, "/", "a")
		encoded = strings.ReplaceAll(encoded, "+", "b")
		return encoded
	}
	digest := gen(bytes)
	if digest[0] <= '9' {
		digest = digest[1:] + string('a'+'9'-digest[0])
	}
	for {
		if len(digest) >= length {
			break
		}
		digest = digest + gen([]byte(digest))
	}
	return digest[:length]
}

func FindJsonPatchForStructs(src, target interface{}) map[string]interface{} {
	srcJson, _ := json.Marshal(src)
	targetJson, _ := json.Marshal(target)
	patchJson, err := jsonpatch.CreateMergePatch(srcJson, targetJson)
	if err != nil {
		panic(err)
	}
	patchMap := map[string]interface{}{}
	if err := json.Unmarshal(patchJson, &patchMap); err != nil {
		panic(err)
	}
	return patchMap
}

func GetJsonChangesAsString(src, target interface{}, filterList ...string) string {
	patchMap := FindJsonPatchForStructs(src, target)
	srcJson, _ := json.Marshal(src)
	srcMap := map[string]interface{}{}
	if err := json.Unmarshal(srcJson, &srcMap); err != nil {
		panic(err)
	}
	var changes []string
loop:
	for key, newVal := range patchMap {
		for _, nk := range filterList {
			if key == nk {
				continue loop
			}
		}
		originalVal := srcMap[key]
		changes = append(changes, fmt.Sprintf("%s(%v=>%v)", key, originalVal, newVal))
	}
	sort.Strings(changes)
	return strings.Join(changes, "|")
}

func MapCompare(mapA, mapB interface{}, intersectionValueFilterFunc func(a, b interface{}) bool) (extra, missing, intersection sets.String) {
	keysetA := StringKeys(mapA)
	keysetB := StringKeys(mapB)
	extra = keysetA.Difference(keysetB)
	missing = keysetB.Difference(keysetA)
	intersection = keysetB.Intersection(keysetA)

	if intersectionValueFilterFunc == nil {
		return
	}
	for _, k := range intersection.List() {
		vA := reflect.ValueOf(mapA).MapIndex(reflect.ValueOf(k)).Interface()
		vB := reflect.ValueOf(mapB).MapIndex(reflect.ValueOf(k)).Interface()
		if intersectionValueFilterFunc(vA, vB) {
			intersection.Delete(k)
		}
	}
	return
}

func StructsAreEqualForKeys(structA, structB interface{}, keys ...string) bool {
	sA := structs.New(structA)
	sB := structs.New(structB)

	if len(keys) == 0 {
		keys = sets.NewString(sA.Names()...).Insert(sB.Names()...).List()
	}

	for _, k := range keys {
		vA, okA := sA.FieldOk(k)
		vB, okB := sB.FieldOk(k)
		if !okA && !okB { // field not present in either of them
			continue
		}
		if !NullOrDeepEqual(vA.Value(), vB.Value()) {
			return false
		}
	}

	return true

}

func Jsonify(obj interface{}) string {
	s, _ := json.Marshal(obj)
	return string(s)
}
