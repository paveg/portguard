package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJsonMarshalIndent(t *testing.T) {
	t.Run("marshal_string", func(t *testing.T) {
		input := "test string"
		result, err := jsonMarshalIndent(input)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Contains(t, string(result), "test string")
	})

	t.Run("marshal_map", func(t *testing.T) {
		input := map[string]interface{}{
			"key1": "value1",
			"key2": 42,
			"key3": true,
		}
		result, err := jsonMarshalIndent(input)
		require.NoError(t, err)
		assert.NotNil(t, result)
		// fmt.Appendf with %+v will produce a Go-formatted map
		resultStr := string(result)
		assert.Contains(t, resultStr, "key1")
		assert.Contains(t, resultStr, "value1")
		assert.Contains(t, resultStr, "42")
		assert.Contains(t, resultStr, "true")
	})

	t.Run("marshal_slice", func(t *testing.T) {
		input := []string{"item1", "item2", "item3"}
		result, err := jsonMarshalIndent(input)
		require.NoError(t, err)
		assert.NotNil(t, result)
		resultStr := string(result)
		assert.Contains(t, resultStr, "item1")
		assert.Contains(t, resultStr, "item2")
		assert.Contains(t, resultStr, "item3")
	})

	t.Run("marshal_struct", func(t *testing.T) {
		type TestStruct struct {
			Name  string
			Value int
			Flag  bool
		}
		input := TestStruct{
			Name:  "test",
			Value: 100,
			Flag:  true,
		}
		result, err := jsonMarshalIndent(input)
		require.NoError(t, err)
		assert.NotNil(t, result)
		resultStr := string(result)
		assert.Contains(t, resultStr, "Name")
		assert.Contains(t, resultStr, "test")
		assert.Contains(t, resultStr, "100")
	})

	t.Run("marshal_nil", func(t *testing.T) {
		result, err := jsonMarshalIndent(nil)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Contains(t, string(result), "<nil>")
	})

	t.Run("marshal_number", func(t *testing.T) {
		input := 42
		result, err := jsonMarshalIndent(input)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "42", string(result))
	})

	t.Run("marshal_boolean", func(t *testing.T) {
		input := true
		result, err := jsonMarshalIndent(input)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "true", string(result))
	})

	t.Run("marshal_empty_map", func(t *testing.T) {
		input := make(map[string]interface{})
		result, err := jsonMarshalIndent(input)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Contains(t, string(result), "map")
	})

	t.Run("marshal_empty_slice", func(t *testing.T) {
		input := []string{}
		result, err := jsonMarshalIndent(input)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Contains(t, string(result), "[]")
	})
}
