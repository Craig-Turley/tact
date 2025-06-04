package idgen

import "github.com/bwmarrin/snowflake"

var sf *snowflake.Node

func Init(node int64) error {
	var err error
	sf, err = snowflake.NewNode(node)
	return err
}

func NewId() snowflake.ID {
	return sf.Generate()
}
