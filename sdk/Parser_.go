package sdk

import (
	"bytes"
	"encoding/xml"
	"io"

	"github.com/BurntSushi/toml"
	jsoniter "github.com/json-iterator/go"
	yaml "gopkg.in/yaml.v3"
)

type Decoder interface{ Decode(v interface{}) error }

type Encoder interface{ Encode(v interface{}) error }

type Parser interface {
	Marshal(v interface{}) (p []byte, err error)
	Unmarshal(p []byte, v interface{}) (err error)
	NewDecoder(r io.Reader) Decoder
	NewEncoder(w io.Writer) Encoder
}

//nolint: gochecknoglobals
var (
	JSON Parser = json_{jsoniter.ConfigFastest}
	TOML Parser = toml_{}
	YAML Parser = yaml_{}
	XML  Parser = xml_{}
)

type json_ struct{ jsoniter.API }

func (json json_) NewDecoder(r io.Reader) Decoder { return json.API.NewDecoder(r) }

func (json json_) NewEncoder(w io.Writer) Encoder { return json.API.NewEncoder(w) }

type toml_ struct{ r io.Reader }

func (toml_) Marshal(v interface{}) ([]byte, error) {
	b := new(bytes.Buffer)
	err := toml.NewEncoder(b).Encode(v)

	return b.Bytes(), err
}

func (toml_) Unmarshal(data []byte, v interface{}) error {
	return toml.Unmarshal(data, v)
}

func (toml_) NewDecoder(r io.Reader) Decoder { return toml_{r} }

func (toml_) NewEncoder(w io.Writer) Encoder { return toml.NewEncoder(w) }

func (t toml_) Decode(v interface{}) error {
	if p, err := io.ReadAll(t.r); err != nil {
		return err
	} else {
		return toml.Unmarshal(p, v)
	}
}

type yaml_ struct{}

func (yaml_) Marshal(v interface{}) ([]byte, error) {
	return yaml.Marshal(v)
}

func (yaml_) Unmarshal(data []byte, v interface{}) error {
	return yaml.Unmarshal(data, v)
}

func (yaml_) NewDecoder(r io.Reader) Decoder { return yaml.NewDecoder(r) }

func (yaml_) NewEncoder(w io.Writer) Encoder { return yaml.NewEncoder(w) }

type xml_ struct{}

func (xml_) Marshal(v interface{}) ([]byte, error) {
	return xml.Marshal(v)
}

func (xml_) Unmarshal(data []byte, v interface{}) error {
	return xml.Unmarshal(data, v)
}

func (xml_) NewDecoder(r io.Reader) Decoder { return xml.NewDecoder(r) }

func (xml_) NewEncoder(w io.Writer) Encoder { return xml.NewEncoder(w) }
