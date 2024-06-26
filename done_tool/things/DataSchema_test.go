package things

import (
	"testing"

	vocab "github.com/hiveot/hub/done_api/api_go"
	"github.com/hiveot/hub/done_tool/ser"

	"github.com/stretchr/testify/assert"
)

// testing of marshalling and unmarshalling schemas

func TestStringSchema(t *testing.T) {
	ss := DataSchema{
		Type:            vocab.WoTDataTypeString,
		StringMinLength: 10,
	}
	enc1, err := ser.Marshal(ss)
	assert.NoError(t, err)
	//
	ds := DataSchema{}
	err = ser.Unmarshal(enc1, &ds)
	assert.NoError(t, err)
}

func TestObjectSchema(t *testing.T) {
	os := DataSchema{
		Type:       vocab.WoTDataTypeObject,
		Properties: make(map[string]DataSchema),
	}
	os.Properties["stringProp"] = DataSchema{
		Type:            vocab.WoTDataTypeString,
		StringMinLength: 10,
	}
	os.Properties["intProp"] = DataSchema{
		Type:          vocab.WoTDataTypeInteger,
		NumberMinimum: 10,
		NumberMaximum: 20,
	}
	enc1, err := ser.Marshal(os)
	assert.NoError(t, err)
	//
	var ds map[string]interface{}
	err = ser.Unmarshal(enc1, &ds)
	assert.NoError(t, err)

	var as DataSchema
	err = ser.Unmarshal(enc1, &as)
	assert.NoError(t, err)

	assert.Equal(t, 10, int(as.Properties["intProp"].NumberMinimum))

}
