package sk

import (
	"encoding/base32"
	"encoding/hex"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"git.parallelcoin.io/pod/pkg/util/base58"
)

// ParseInt tries to read an integer from a string
func ParseInt(in string) (out Int, err error) {
	var i int64
	i, err = strconv.ParseInt(in, 10, 64)
	return Int(i), err
}

// ParseFloat tries to read a floating point number from a string
func ParseFloat(in string) (out Float, err error) {
	var i float64
	i, err = strconv.ParseFloat(in, 64)
	return Float(i), err
}

// ParseDuration takes a string and tries to read a duration in Golang time.Duration format
func ParseDuration(in string) (out Duration, err error) {
	o, err := time.ParseDuration(in)
	return Duration(o), err
}

// ParseTime takes a string and tries to read a time specification from it (time of day) as simple 24 hour HH:MM:SS format
func ParseTime(in string) (out Time, err error) {
	t, err := time.Parse("15:04:05", in)
	return Time(t), err
}

// ParseDate takes a string and tries to read a date in yyyy-mm-dd format
func ParseDate(in string) (out Date, err error) {
	t, err := time.Parse("2006-01-02", in)
	return Date(t), err
}

const (
	zero int64 = 0
	one  int64 = 1
	kB   int64 = 1024
	mB   int64 = 1024 * kB
	gB   int64 = 1024 * mB
	tB   int64 = 1024 * gB
	pB   int64 = 1024 * tB

	kiB int64 = 1000
	miB int64 = 1000 * kiB
	giB int64 = 1000 * miB
	tiB int64 = 1000 * giB
	piB int64 = 1000 * tiB
)

// ParseSize accepts a string and returns a value representing bytes, using the following annotations:
// kKmMgGtTpP single letter for power of 2 based size
// kb/mb/gb/tb/pb case insensitive ^2 based size
// kib/mib/gib/tib/pib case insensitive 10 based size
func ParseSize(in string) (out Size, err error) {
	unit := one
	var ii int64
	ii, err = strconv.ParseInt(in, 10, 64)
	if err == nil {
		return Size(ii), nil
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
			out = Size(unit * ii)
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
			out = Size(unit * ii)
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
			out = Size(unit * ii)
		}
		return
	four:
		err = errors.New("did not decode a size from the string")
	}
	return
}

// ParseString takes a string and returns a String (and never an error)
func ParseString(in string) (out String, err error) {
	return String(in), nil
}

// ParseURL takes a string and tries to construct a full URL from it based on assuming http from first slash after domain name, can include a port
func ParseURL(in string) (out Url, err error) {
	var u *url.URL
	u, err = url.Parse(in)
	if err == nil {
		out = Url(u.String())
	}
	return
}

// ParseAddress takes a string and tries to get an IPv4 or IPv6 address
func ParseAddress(in string) (out Address, err error) {
	var u *url.URL
	u, err = url.Parse(in)
	if err == nil {
		scheme := u.Scheme
		host := u.Host
		out = Address(scheme + "://" + host)
	}
	return
}

// ParseBase58 takes a string and tries to read a base58check binary value
func ParseBase58(in string) (out Base58, err error) {
	var r []byte
	var v byte
	r, v, err = base58.CheckDecode(in)
	return append(append(out, v), r...), err
}

// ParseBase32 takes a string and tries to read base32 from it
func ParseBase32(in string) (out Base32, err error) {
	var b []byte
	b, err = base32.StdEncoding.DecodeString(strings.ToUpper(in))
	if err == nil {
		out = Base32(b)
	}
	return
}

// ParseHex takes a string and tries to read a hex string, including potentially the 0x prefix
func ParseHex(in string) (out Hex, err error) {
	var b []byte
	b, err = hex.DecodeString(in)
	if err == nil {
		out = Hex(b)
	}
	return
}
