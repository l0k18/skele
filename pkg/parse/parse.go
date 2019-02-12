package parse

import (
	"encoding/base32"
	"encoding/hex"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"git.parallelcoin.io/pod/pkg/util/base58"
	"github.com/l0k1verloren/skele/pkg/T"
)

// Int tries to read an integer from a string
func Int(in string) (out T.Int, err error) {
	var i int64
	i, err = strconv.ParseInt(in, 10, 64)
	return T.Int(i), err
}

// Float tries to read a floating point number from a string
func Float(in string) (out T.Float, err error) {
	var i float64
	i, err = strconv.ParseFloat(in, 64)
	return T.Float(i), err
}

// Duration takes a string and tries to read a duration in Golang time.Duration format
func Duration(in string) (out T.Duration, err error) {
	o, err := time.ParseDuration(in)
	return T.Duration(o), err
}

// Time takes a string and tries to read a time specification from it (time of day) as simple 24 hour HH:MM:SS format
func Time(in string) (out T.Time, err error) {
	t, err := time.Parse("15:04:05", in)
	return T.Time(t), err
}

// Date takes a string and tries to read a date in yyyy-mm-dd format
func Date(in string) (out T.Date, err error) {
	t, err := time.Parse("2006-01-02", in)
	return T.Date(t), err
}

const (
	zero int64 = 0
	one        = zero + 1
	kB         = one * 1024
	mB         = kB * kB
	gB         = mB * kB
	tB         = gB * kB
	pB         = tB * kB

	kiB int64 = 1000
	miB int64 = 1000 * kiB
	giB int64 = 1000 * miB
	tiB int64 = 1000 * giB
	piB int64 = 1000 * tiB
)

// Size accepts a string and returns a value representing bytes, using the following annotations:
// kKmMgGtTpP single letter for power of 2 based size
// kb/mb/gb/tb/pb case insensitive ^2 based size
// kib/mib/gib/tib/pib case insensitive 10 based size
func Size(in string) (out T.Size, err error) {
	unit := one
	var ii int64
	ii, err = strconv.ParseInt(in, 10, 64)
	if err == nil {
		return T.Size(ii), nil
	}
	if len(in) > 1 {
		last1 := in[len(in)-1]
		switch last1 {
		case 'k', 'K':
			unit = kB
		case 'm', 'M':
			unit = mB
		case 'g', 'G':
			unit = gB
		case 't', 'T':
			unit = tB
		case 'p', 'P':
			unit = pB
		default:
			goto two
		}
		ii, err = strconv.ParseInt(in[:len(in)-1], 10, 64)
		if err == nil {
			out = T.Size(unit * ii)
		}
		return
	}
two:
	if len(in) > 2 {
		last2 := strings.ToLower(in[len(in)-2 : len(in)-1])
		switch last2 {
		case "kb":
			unit = kB
		case "mb":
			unit = mB
		case "gb":
			unit = gB
		case "tb":
			unit = tB
		case "pb":
			unit = pB
		default:
			goto three
		}
		ii, err = strconv.ParseInt(in[:len(in)-2], 10, 64)
		if err == nil {
			out = T.Size(unit * ii)
		}
		return
	}
three:
	if len(in) > 3 {
		last3 := strings.ToLower(in[len(in)-3 : len(in)-1])
		switch last3 {
		case "kb":
			unit = kiB
		case "mb":
			unit = miB
		case "gb":
			unit = giB
		case "tb":
			unit = tiB
		case "pb":
			unit = piB
		default:
			goto four
		}
		ii, err = strconv.ParseInt(in[:len(in)-3], 10, 64)
		if err == nil {
			out = T.Size(unit * ii)
		}
		return
	four:
		err = errors.New("did not decode a size from the string")
	}
	return
}

// String takes a string and returns a String (and never an error)
func String(in string) (out T.String, err error) {
	return T.String(in), nil
}

// URL takes a string and tries to construct a full URL from it based on assuming http from first slash after domain name, can include a port
func URL(in string) (out T.Url, err error) {
	var u *url.URL
	u, err = url.Parse(in)
	if err == nil {
		out = T.Url(u.String())
	}
	return
}

// Address takes a string and tries to get an IPv4 or IPv6 address
func Address(in string) (out T.Address, err error) {
	var u *url.URL
	u, err = url.Parse(in)
	if err == nil {
		scheme := u.Scheme
		host := u.Host
		out = T.Address(scheme + "://" + host)
	}
	return
}

// Base58 takes a string and tries to read a base58check binary value
func Base58(in string) (out T.Base58, err error) {
	var r []byte
	var v byte
	r, v, err = base58.CheckDecode(in)
	return append(append(out, v), r...), err
}

// Base32 takes a string and tries to read base32 from it
func Base32(in string) (out T.Base32, err error) {
	var b []byte
	b, err = base32.StdEncoding.DecodeString(strings.ToUpper(in))
	if err == nil {
		out = T.Base32(b)
	}
	return
}

// Hex takes a string and tries to read a hex string, including potentially the 0x prefix
func Hex(in string) (out T.Hex, err error) {
	var b []byte
	b, err = hex.DecodeString(in)
	if err == nil {
		out = T.Hex(b)
	}
	return
}

// ToType takes a string and a variable and attempts to decode the value according to the type of the variable
func ToType(in string, I interface{}) (out interface{}, err error) {
	switch I.(type) {
	case T.Int:
		var o T.Int
		if o, err = Int(in); err == nil {
			out = o
		}
	case T.Float:
		var o T.Float
		if o, err = Float(in); err == nil {
			out = o
		}
	case T.Duration:
		var o T.Duration
		if o, err = Duration(in); err == nil {
			out = o
		}
	case T.Time:
		var o T.Time
		if o, err = Time(in); err == nil {
			out = o
		}
	case T.Date:
		var o T.Date
		if o, err = Date(in); err == nil {
			out = o
		}
	case T.Size:
		var o T.Size
		if o, err = Size(in); err == nil {
			out = o
		}
	case T.String:
		var o T.String
		if o, err = String(in); err == nil {
			out = o
		}
	case T.Url:
		var o T.Url
		if o, err = URL(in); err == nil {
			out = o
		}
	case T.Address:
		var o T.Address
		if o, err = Address(in); err == nil {
			out = o
		}
	case T.Base58:
		var o T.Base58
		if o, err = Base58(in); err == nil {
			out = o
		}
	case T.Base32:
		var o T.Base32
		if o, err = Base32(in); err == nil {
			out = o
		}
	case T.Hex:
		var o T.Hex
		if o, err = Hex(in); err == nil {
			out = o
		}
	default:
		err = errors.New("unhandled type")
	}
	return
}
