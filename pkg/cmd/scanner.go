package cmd

import (
	"fmt"
)

// Scan is the parser, searches and fills variables and invokes scanner on child commands, runs functions if command has one
func (c *command) Scan(args []string) error {
	fmt.Print("scanning ")
	// fmt.Println(c.Path())
	// crsr := c.Cursor()
	// for crsr.Next() {
	// 	p := crsr.Item()
	// 	switch p.Name() {
	// 	case Types[INT].K:
	// 		fmt.Println(p.K, p.V)
	// 	case Types[INTLIST].K:
	// 		fmt.Println(p.K, p.V)
	// 	case Types[FLOAT].K:
	// 		fmt.Println(p.K, p.V)
	// 	case Types[DURATION].K:
	// 		fmt.Println(p.K, p.V)
	// 	case Types[TIME].K:
	// 		fmt.Println(p.K, p.V)
	// 	case Types[TIMELIST].K:
	// 		fmt.Println(p.K, p.V)
	// 	case Types[DATE].K:
	// 		fmt.Println(p.K, p.V)
	// 	case Types[DATELIST].K:
	// 		fmt.Println(p.K, p.V)
	// 	case Types[SIZE].K:
	// 		fmt.Println(p.K, p.V)
	// 	case Types[SIZELIST].K:
	// 		fmt.Println(p.K, p.V)
	// 	case Types[STRING].K:
	// 		fmt.Println(p.K, p.V)
	// 	case Types[STRINGLIST].K:
	// 		fmt.Println(p.K, p.V)
	// 	case Types[URL].K:
	// 		fmt.Println(p.K, p.V)
	// 	case Types[URLLIST].K:
	// 		fmt.Println(p.K, p.V)
	// 	case Types[ADDRESS].K:
	// 		fmt.Println(p.K, p.V)
	// 	case Types[ADDRESSLIST].K:
	// 		fmt.Println(p.K, p.V)
	// 	case Types[BASE58].K:
	// 		fmt.Println(p.K, p.V)
	// 	case Types[BASE58LIST].K:
	// 		fmt.Println(p.K, p.V)
	// 	case Types[BASE32].K:
	// 		fmt.Println(p.K, p.V)
	// 	case Types[BASE32LIST].K:
	// 		fmt.Println(p.K, p.V)
	// 	case Types[HEX].K:
	// 		fmt.Println(p.K, p.V)
	// 	case Types[HEXLIST].K:
	// 		fmt.Println(p.K, p.V)
	// 	case Types[COMMAND].K:
	// 		fmt.Println(p.K, p.V)
	// 		p.V.(Commander).Scan(args)
	// 	}
	// }
	return nil
}

// Match returns true if the whole token is the same as the first 1-4 characters of the key
func Match(key, token string) (b bool) {
	if key == token[:1] ||
		key == token[:2] ||
		key == token[:3] ||
		key == token[:4] {
		b = true
	}
	return
}
