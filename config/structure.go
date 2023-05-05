package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/intxff/rdcross/dns"
	"github.com/intxff/rdcross/egress"
	eg "github.com/intxff/rdcross/egress/general"
	"github.com/intxff/rdcross/ingress"
	ig "github.com/intxff/rdcross/ingress/general"
	"github.com/intxff/rdcross/ingress/tun"
	"github.com/intxff/rdcross/log"
	"github.com/intxff/rdcross/util"
	"gopkg.in/yaml.v3"
)

// config structure to unmarshal yaml
type RdConfig struct {
	Ingress      Ingresses    `yaml:"ingress"`
	IngressGroup IngressGroup `yaml:"ingress_group"`
	Egress       Egresses     `yaml:"egress"`
	EgressGroup  EgressGroup  `yaml:"egress_group"`
	Rule         []string     `yaml:"rule"`
	DNS          dns.DNS      `yaml:"dns"`
	Log          log.Log      `yaml:"log"`
	Path         string
	Dir          string
}

type Ingresses []ingress.Ingress

func (ing *Ingresses) UnmarshalYAML(value *yaml.Node) error {
	var (
		err error
	)
	for i := 0; i < len(value.Content); i++ {
		temp := make(map[string]interface{})
		if err = value.Content[i].Decode(&temp); err != nil {
			return err
		}

		var (
			aType    ingress.IngressType
			attrMust = map[string]any{
				"type": &aType,
			}
		)
		if err = util.MustHave(temp, attrMust); err != nil {
			return err
		}

		switch ingress.IngressType(strings.ToUpper(string(aType))) {
		case ingress.TypeGeneral:
			ingTemp := &ig.General{}
			if err = value.Content[i].Decode(&ingTemp); err != nil {
				return err
			}
			*ing = append(*ing, ingTemp)
		case ingress.TypeTun:
			ingTemp := &tun.Tun{}
			if err = value.Content[i].Decode(&ingTemp); err != nil {
				return err
			}
			*ing = append(*ing, ingTemp)
		default:
			return fmt.Errorf("invalid ingress type %v", aType)
		}
	}

	return nil
}

func (in Ingresses) Value() []string {
	r := make([]string, len(in))
	if len(in) == 0 {
		return nil
	}

	for i := 0; i < len(in); i++ {
		r[i] = in[i].Name()
	}
	return r
}

func (in Ingresses) Members() [][]string {
	return nil
}

type IngressGroup []struct {
	Name   string
	Member []string
}

func (in IngressGroup) Value() []string {
	r := make([]string, len(in))
	if len(in) == 0 {
		return nil
	}

	for i := 0; i < len(in); i++ {
		r[i] = in[i].Name
	}
	return r
}

func (in IngressGroup) Members() [][]string {
	r := make([][]string, len(in))
	if len(in) == 0 {
		return nil
	}

	for i := 0; i < len(in); i++ {
		r[i] = make([]string, len(in[i].Member))
		copy(r[i], in[i].Member)
	}
	return r
}

type Egresses []egress.Egress

func (egr *Egresses) UnmarshalYAML(value *yaml.Node) error {
	var (
		err error
	)
	for i := 0; i < len(value.Content); i++ {
		temp := make(map[string]interface{})
		if err = value.Content[i].Decode(&temp); err != nil {
			return err
		}

		var (
			aType    egress.EgressType
			attrMust = map[string]any{
				"type": &aType,
			}
		)
		if err = util.MustHave(temp, attrMust); err != nil {
			return err
		}

		switch egress.EgressType(strings.ToUpper(string(aType))) {
		case egress.TypeGeneral:
			egrTemp := &eg.General{}
			if err := value.Content[i].Decode(&egrTemp); err != nil {
				return err
			}
			*egr = append(*egr, egrTemp)
		default:
			return errors.New("invalid egress type")
		}
	}
	return nil
}

func (e Egresses) Value() []string {
	r := make([]string, len(e))
	if len(e) == 0 {
		return nil
	}

	for i := 0; i < len(e); i++ {
		r[i] = e[i].Name()
	}
	return r
}

func (e Egresses) Members() [][]string {
	return nil
}

type EgressGroup []struct {
	Name   string
	Member []string
}

func (e EgressGroup) Value() []string {
	r := make([]string, len(e))
	if len(e) == 0 {
		return nil
	}

	for i := 0; i < len(e); i++ {
		r[i] = e[i].Name
	}
	return r
}

func (e EgressGroup) Members() [][]string {
	r := make([][]string, len(e))
	if len(e) == 0 {
		return nil
	}

	for i := 0; i < len(e); i++ {
		r[i] = make([]string, len(e[i].Member))
		copy(r[i], e[i].Member)
	}
	return r
}
