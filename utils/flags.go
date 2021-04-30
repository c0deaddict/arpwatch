package utils

import (
	"errors"
	"flag"
	"strconv"
	"strings"
)

type stringSliceFlag struct {
	value []string
}

func (i *stringSliceFlag) String() string {
	return strings.Join(i.value, ",")
}

func (i *stringSliceFlag) Set(value string) error {
	i.value = append(i.value, strings.TrimSpace(value))
	return nil
}

func StringSliceFlag(name string, value []string, usage string) *[]string {
	slice := stringSliceFlag{value}
	flag.Var(&slice, name, usage)
	return &slice.value
}

type portFlag struct{ value uint16 }

func (p *portFlag) String() string {
	return strconv.Itoa(int(p.value))
}

func (p *portFlag) Set(value string) error {
	port, err := strconv.Atoi(value)
	if err != nil {
		return err
	}

	if port < 1 || port > 65535 {
		return errors.New("Port must be in range 1-65535")
	}

	p.value = uint16(port)
	return nil
}

func PortFlag(name string, value uint16, usage string) *uint16 {
	port := portFlag{value}
	flag.Var(&port, name, usage)
	return &port.value
}
