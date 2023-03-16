package abugo

type GameMaker struct {
	maketype int
}

func (c *GameMaker) Init(maketype int) {
	c.maketype = maketype
}
