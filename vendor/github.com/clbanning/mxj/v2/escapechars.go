// Copyright 2016 Charles Banning. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file

package mxj

import (
	"bytes"
)

var xmlEscapeChars bool

// XMLEscapeChars(true) forces escaping invalid characters in attribute and element values.
// NOTE: this is brute force with NO interrogation of '&' being escaped already; if it is
// then '&amp;' will be re-escaped as '&amp;amp;'.
//
/*
	The values are:
	"   &quot;
	'   &apos;
	<   &lt;
	>   &gt;
	&   &amp;
*/
//
// Note: if XMLEscapeCharsDecoder(true) has been called - or the default, 'false,' value
// has been toggled to 'true' - then XMLEscapeChars(true) is ignored.  If XMLEscapeChars(true)
// has already been called before XMLEscapeCharsDecoder(true), XMLEscapeChars(false) is called
// to turn escape encoding on mv.Xml, etc., to prevent double escaping ampersands, '&'.
func XMLEscapeChars(b ...bool) {
	var bb bool
	if len(b) == 0 {
		bb = !xmlEscapeChars
	} else {
		bb = b[0]
	}
	if bb == true && xmlEscapeCharsDecoder == false {
		xmlEscapeChars = true
	} else {
		xmlEscapeChars = false
	}
}

// Scan for '&' first, since 's' may contain "&amp;" that is parsed to "&amp;amp;"
// - or "&lt;" that is parsed to "&amp;lt;".
var escapechars = [][2][]byte{
	{[]byte(`&`), []byte(`&amp;`)},
	{[]byte(`<`), []byte(`&lt;`)},
	{[]byte(`>`), []byte(`&gt;`)},
	{[]byte(`"`), []byte(`&quot;`)},
	{[]byte(`'`), []byte(`&apos;`)},
}

func escapeChars(s string) string {
	if len(s) == 0 {
		return s
	}

	b := []byte(s)
	for _, v := range escapechars {
		n := bytes.Count(b, v[0])
		if n == 0 {
			continue
		}
		b = bytes.Replace(b, v[0], v[1], n)
	}
	return string(b)
}

// per issue #84, escape CharData values from xml.Decoder

var xmlEscapeCharsDecoder bool

// XMLEscapeCharsDecoder(b ...bool) escapes XML characters in xml.CharData values
// returned by Decoder.Token.  Thus, the internal Map values will contain escaped
// values, and you do not need to set XMLEscapeChars for proper encoding.
//
// By default, the Map values have the non-escaped values returned by Decoder.Token.
// XMLEscapeCharsDecoder(true) - or, XMLEscapeCharsDecoder() - will toggle escape
// encoding 'on.'
//
// Note: if XMLEscapeCharDecoder(true) is call then XMLEscapeChars(false) is
// called to prevent re-escaping the values on encoding using mv.Xml, etc.
func XMLEscapeCharsDecoder(b ...bool) {
	if len(b) == 0 {
		xmlEscapeCharsDecoder = !xmlEscapeCharsDecoder
	} else {
		xmlEscapeCharsDecoder = b[0]
	}
	if xmlEscapeCharsDecoder == true && xmlEscapeChars == true {
		xmlEscapeChars = false
	}
}
