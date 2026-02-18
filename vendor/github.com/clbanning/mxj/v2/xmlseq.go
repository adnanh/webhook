// Copyright 2012-2016, 2019 Charles Banning. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file

// xmlseq.go - version of xml.go with sequence # injection on Decoding and sorting on Encoding.
// Also, handles comments, directives and process instructions.

package mxj

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
)

// MapSeq is like Map but contains seqencing indices to allow recovering the original order of
// the XML elements when the map[string]interface{} is marshaled. Element attributes are
// stored as a map["#attr"]map[<attr_key>]map[string]interface{}{"#text":"<value>", "#seq":<attr_index>}
// value instead of denoting the keys with a prefix character.  Also, comments, directives and
// process instructions are preserved.
type MapSeq map[string]interface{}

// NoRoot is returned by NewXmlSeq, etc., when a comment, directive or procinstr element is parsed
// in the XML data stream and the element is not contained in an XML object with a root element.
var NoRoot = errors.New("no root key")
var NO_ROOT = NoRoot // maintain backwards compatibility

// ------------------- NewMapXmlSeq & NewMapXmlSeqReader ... -------------------------

// NewMapXmlSeq converts a XML doc into a MapSeq value with elements id'd with decoding sequence key represented
// as map["#seq"]<int value>.
// If the optional argument 'cast' is 'true', then values will be converted to boolean or float64 if possible.
// NOTE: "#seq" key/value pairs are removed on encoding with msv.Xml() / msv.XmlIndent().
//	• attributes are a map - map["#attr"]map["attr_key"]map[string]interface{}{"#text":<aval>, "#seq":<num>}
//	• all simple elements are decoded as map["#text"]interface{} with a "#seq" k:v pair, as well.
//	• lists always decode as map["list_tag"][]map[string]interface{} where the array elements are maps that
//	  include a "#seq" k:v pair based on sequence they are decoded.  Thus, XML like:
//	      <doc>
//	         <ltag>value 1</ltag>
//	         <newtag>value 2</newtag>
//	         <ltag>value 3</ltag>
//	      </doc>
//	  is decoded as:
//	    doc :
//	      ltag :[[]interface{}]
//	        [item: 0]
//	          #seq :[int] 0
//	          #text :[string] value 1
//	        [item: 1]
//	          #seq :[int] 2
//	          #text :[string] value 3
//	      newtag :
//	        #seq :[int] 1
//	        #text :[string] value 2
//	  It will encode in proper sequence even though the MapSeq representation merges all "ltag" elements in an array.
//	• comments - "<!--comment-->" -  are decoded as map["#comment"]map["#text"]"cmnt_text" with a "#seq" k:v pair.
//	• directives - "<!text>" - are decoded as map["#directive"]map[#text"]"directive_text" with a "#seq" k:v pair.
//	• process instructions  - "<?instr?>" - are decoded as map["#procinst"]interface{} where the #procinst value
//	  is of map[string]interface{} type with the following keys: #target, #inst, and #seq.
//	• comments, directives, and procinsts that are NOT part of a document with a root key will be returned as
//	  map[string]interface{} and the error value 'NoRoot'.
//	• note: "<![CDATA[" syntax is lost in xml.Decode parser - and is not handled here, either.
//	   and: "\r\n" is converted to "\n"
//
//	NOTES:
//	   1. The 'xmlVal' will be parsed looking for an xml.StartElement, xml.Comment, etc., so BOM and other
//	      extraneous xml.CharData will be ignored unless io.EOF is reached first.
//	   2. CoerceKeysToLower() is NOT recognized, since the intent here is to eventually call m.XmlSeq() to
//	      re-encode the message in its original structure.
//	   3. If CoerceKeysToSnakeCase() has been called, then all key values will be converted to snake case.
//
//	NAME SPACES:
//	   1. Keys in the MapSeq value that are parsed from a <name space prefix>:<local name> tag preserve the
//	      "<prefix>:" notation rather than stripping it as with NewMapXml().
//	   2. Attribute keys for name space prefix declarations preserve "xmlns:<prefix>" notation.
//
//	ERRORS:
//	   1. If a NoRoot error, "no root key," is returned, check the initial map key for a "#comment",
//	      "#directive" or #procinst" key.
//	   2. Unmarshaling an XML doc that is formatted using the whitespace character, " ", will error, since
//	      Decoder.RawToken treats such occurances as significant. See NewMapFormattedXmlSeq().
func NewMapXmlSeq(xmlVal []byte, cast ...bool) (MapSeq, error) {
	var r bool
	if len(cast) == 1 {
		r = cast[0]
	}
	return xmlSeqToMap(xmlVal, r)
}

// NewMapFormattedXmlSeq performs the same as NewMapXmlSeq but is useful for processing XML objects that
// are formatted using the whitespace character, " ".  (The stdlib xml.Decoder, by default, treats all
// whitespace as significant; Decoder.Token() and Decoder.RawToken() will return strings of one or more
// whitespace characters and without alphanumeric or punctuation characters as xml.CharData values.)
//
// If you're processing such XML, then this will convert all occurrences of whitespace-only strings
// into an empty string, "", prior to parsing the XML - irrespective of whether the occurrence is
// formatting or is a actual element value.
func NewMapFormattedXmlSeq(xmlVal []byte, cast ...bool) (MapSeq, error) {
	var c bool
	if len(cast) == 1 {
		c = cast[0]
	}

	// Per PR #104 - clean out formatting characters so they don't show up in Decoder.RawToken() stream.
	// NOTE: Also replaces element values that are solely comprised of formatting/whitespace characters
	// with empty string, "".
	r := regexp.MustCompile(`>[\n\t\r ]*<`)
	xmlVal = r.ReplaceAll(xmlVal, []byte("><"))
	return xmlSeqToMap(xmlVal, c)
}

// NewMpaXmlSeqReader returns next XML doc from an io.Reader as a MapSeq value.
//	NOTES:
//	   1. The 'xmlReader' will be parsed looking for an xml.StartElement, xml.Comment, etc., so BOM and other
//	      extraneous xml.CharData will be ignored unless io.EOF is reached first.
//	   2. CoerceKeysToLower() is NOT recognized, since the intent here is to eventually call m.XmlSeq() to
//	      re-encode the message in its original structure.
//	   3. If CoerceKeysToSnakeCase() has been called, then all key values will be converted to snake case.
//
//	ERRORS:
//	   1. If a NoRoot error, "no root key," is returned, check the initial map key for a "#comment",
//	      "#directive" or #procinst" key.
func NewMapXmlSeqReader(xmlReader io.Reader, cast ...bool) (MapSeq, error) {
	var r bool
	if len(cast) == 1 {
		r = cast[0]
	}

	// We need to put an *os.File reader in a ByteReader or the xml.NewDecoder
	// will wrap it in a bufio.Reader and seek on the file beyond where the
	// xml.Decoder parses!
	if _, ok := xmlReader.(io.ByteReader); !ok {
		xmlReader = myByteReader(xmlReader) // see code at EOF
	}

	// build the map
	return xmlSeqReaderToMap(xmlReader, r)
}

// NewMapXmlSeqReaderRaw returns the  next XML doc from  an io.Reader as a MapSeq value.
// Returns MapSeq value, slice with the raw XML, and any error.
//	NOTES:
//	   1. Due to the implementation of xml.Decoder, the raw XML off the reader is buffered to []byte
//	      using a ByteReader. If the io.Reader is an os.File, there may be significant performance impact.
//	      See the examples - getmetrics1.go through getmetrics4.go - for comparative use cases on a large
//	      data set. If the io.Reader is wrapping a []byte value in-memory, however, such as http.Request.Body
//	      you CAN use it to efficiently unmarshal a XML doc and retrieve the raw XML in a single call.
//	    2. The 'raw' return value may be larger than the XML text value.
//	    3. The 'xmlReader' will be parsed looking for an xml.StartElement, xml.Comment, etc., so BOM and other
//	       extraneous xml.CharData will be ignored unless io.EOF is reached first.
//	    4. CoerceKeysToLower() is NOT recognized, since the intent here is to eventually call m.XmlSeq() to
//	       re-encode the message in its original structure.
//	    5. If CoerceKeysToSnakeCase() has been called, then all key values will be converted to snake case.
//
//	ERRORS:
//	    1. If a NoRoot error, "no root key," is returned, check if the initial map key is "#comment",
//	       "#directive" or #procinst" key.
func NewMapXmlSeqReaderRaw(xmlReader io.Reader, cast ...bool) (MapSeq, []byte, error) {
	var r bool
	if len(cast) == 1 {
		r = cast[0]
	}
	// create TeeReader so we can retrieve raw XML
	buf := make([]byte, 0)
	wb := bytes.NewBuffer(buf)
	trdr := myTeeReader(xmlReader, wb)

	m, err := xmlSeqReaderToMap(trdr, r)

	// retrieve the raw XML that was decoded
	b := wb.Bytes()

	// err may be NoRoot
	return m, b, err
}

// xmlSeqReaderToMap() - parse a XML io.Reader to a map[string]interface{} value
func xmlSeqReaderToMap(rdr io.Reader, r bool) (map[string]interface{}, error) {
	// parse the Reader
	p := xml.NewDecoder(rdr)
	if CustomDecoder != nil {
		useCustomDecoder(p)
	} else {
		p.CharsetReader = XmlCharsetReader
	}
	return xmlSeqToMapParser("", nil, p, r)
}

// xmlSeqToMap - convert a XML doc into map[string]interface{} value
func xmlSeqToMap(doc []byte, r bool) (map[string]interface{}, error) {
	b := bytes.NewReader(doc)
	p := xml.NewDecoder(b)
	if CustomDecoder != nil {
		useCustomDecoder(p)
	} else {
		p.CharsetReader = XmlCharsetReader
	}
	return xmlSeqToMapParser("", nil, p, r)
}

// ===================================== where the work happens =============================

// xmlSeqToMapParser - load a 'clean' XML doc into a map[string]interface{} directly.
// Add #seq tag value for each element decoded - to be used for Encoding later.
func xmlSeqToMapParser(skey string, a []xml.Attr, p *xml.Decoder, r bool) (map[string]interface{}, error) {
	if snakeCaseKeys {
		skey = strings.Replace(skey, "-", "_", -1)
	}

	// NOTE: all attributes and sub-elements parsed into 'na', 'na' is returned as value for 'skey' in 'n'.
	var n, na map[string]interface{}
	var seq int // for including seq num when decoding

	// Allocate maps and load attributes, if any.
	// NOTE: on entry from NewMapXml(), etc., skey=="", and we fall through
	//       to get StartElement then recurse with skey==xml.StartElement.Name.Local
	//       where we begin allocating map[string]interface{} values 'n' and 'na'.
	if skey != "" {
		// 'n' only needs one slot - save call to runtime•hashGrow()
		// 'na' we don't know
		n = make(map[string]interface{}, 1)
		na = make(map[string]interface{})
		if len(a) > 0 {
			// xml.Attr is decoded into: map["#attr"]map[<attr_label>]interface{}
			// where interface{} is map[string]interface{}{"#text":<attr_val>, "#seq":<attr_seq>}
			aa := make(map[string]interface{}, len(a))
			for i, v := range a {
				if snakeCaseKeys {
					v.Name.Local = strings.Replace(v.Name.Local, "-", "_", -1)
				}
				if xmlEscapeCharsDecoder { // per issue#84
					v.Value = escapeChars(v.Value)
				}
				if len(v.Name.Space) > 0 {
					aa[v.Name.Space+`:`+v.Name.Local] = map[string]interface{}{textK: cast(v.Value, r, ""), seqK: i}
				} else {
					aa[v.Name.Local] = map[string]interface{}{textK: cast(v.Value, r, ""), seqK: i}
				}
			}
			na[attrK] = aa
		}
	}

	// Return XMPP <stream:stream> message.
	if handleXMPPStreamTag && skey == "stream:stream" {
		n[skey] = na
		return n, nil
	}

	for {
		t, err := p.RawToken()
		if err != nil {
			if err != io.EOF {
				return nil, errors.New("xml.Decoder.Token() - " + err.Error())
			}
			return nil, err
		}
		switch t.(type) {
		case xml.StartElement:
			tt := t.(xml.StartElement)

			// First call to xmlSeqToMapParser() doesn't pass xml.StartElement - the map key.
			// So when the loop is first entered, the first token is the root tag along
			// with any attributes, which we process here.
			//
			// Subsequent calls to xmlSeqToMapParser() will pass in tag+attributes for
			// processing before getting the next token which is the element value,
			// which is done above.
			if skey == "" {
				if len(tt.Name.Space) > 0 {
					return xmlSeqToMapParser(tt.Name.Space+`:`+tt.Name.Local, tt.Attr, p, r)
				} else {
					return xmlSeqToMapParser(tt.Name.Local, tt.Attr, p, r)
				}
			}

			// If not initializing the map, parse the element.
			// len(nn) == 1, necessarily - it is just an 'n'.
			var nn map[string]interface{}
			if len(tt.Name.Space) > 0 {
				nn, err = xmlSeqToMapParser(tt.Name.Space+`:`+tt.Name.Local, tt.Attr, p, r)
			} else {
				nn, err = xmlSeqToMapParser(tt.Name.Local, tt.Attr, p, r)
			}
			if err != nil {
				return nil, err
			}

			// The nn map[string]interface{} value is a na[nn_key] value.
			// We need to see if nn_key already exists - means we're parsing a list.
			// This may require converting na[nn_key] value into []interface{} type.
			// First, extract the key:val for the map - it's a singleton.
			var key string
			var val interface{}
			for key, val = range nn {
				break
			}

			// add "#seq" k:v pair -
			// Sequence number included even in list elements - this should allow us
			// to properly resequence even something goofy like:
			//     <list>item 1</list>
			//     <subelement>item 2</subelement>
			//     <list>item 3</list>
			// where all the "list" subelements are decoded into an array.
			switch val.(type) {
			case map[string]interface{}:
				val.(map[string]interface{})[seqK] = seq
				seq++
			case interface{}: // a non-nil simple element: string, float64, bool
				v := map[string]interface{}{textK: val, seqK: seq}
				seq++
				val = v
			}

			// 'na' holding sub-elements of n.
			// See if 'key' already exists.
			// If 'key' exists, then this is a list, if not just add key:val to na.
			if v, ok := na[key]; ok {
				var a []interface{}
				switch v.(type) {
				case []interface{}:
					a = v.([]interface{})
				default: // anything else - note: v.(type) != nil
					a = []interface{}{v}
				}
				a = append(a, val)
				na[key] = a
			} else {
				na[key] = val // save it as a singleton
			}
		case xml.EndElement:
			if skey != "" {
				tt := t.(xml.EndElement)
				if snakeCaseKeys {
					tt.Name.Local = strings.Replace(tt.Name.Local, "-", "_", -1)
				}
				var name string
				if len(tt.Name.Space) > 0 {
					name = tt.Name.Space + `:` + tt.Name.Local
				} else {
					name = tt.Name.Local
				}
				if skey != name {
					return nil, fmt.Errorf("element %s not properly terminated, got %s at #%d",
						skey, name, p.InputOffset())
				}
			}
			// len(n) > 0 if this is a simple element w/o xml.Attrs - see xml.CharData case.
			if len(n) == 0 {
				// If len(na)==0 we have an empty element == "";
				// it has no xml.Attr nor xml.CharData.
				// Empty element content will be  map["etag"]map["#text"]""
				// after #seq injection - map["etag"]map["#seq"]seq - after return.
				if len(na) > 0 {
					n[skey] = na
				} else {
					n[skey] = "" // empty element
				}
			}
			return n, nil
		case xml.CharData:
			// clean up possible noise
			tt := strings.Trim(string(t.(xml.CharData)), trimRunes)
			if xmlEscapeCharsDecoder { // issue#84
				tt = escapeChars(tt)
			}
			if skey == "" {
				// per Adrian (http://www.adrianlungu.com/) catch stray text
				// in decoder stream -
				// https://github.com/clbanning/mxj/pull/14#issuecomment-182816374
				// NOTE: CharSetReader must be set to non-UTF-8 CharSet or you'll get
				// a p.Token() decoding error when the BOM is UTF-16 or UTF-32.
				continue
			}
			if len(tt) > 0 {
				// every simple element is a #text and has #seq associated with it
				na[textK] = cast(tt, r, "")
				na[seqK] = seq
				seq++
			}
		case xml.Comment:
			if n == nil { // no root 'key'
				n = map[string]interface{}{commentK: string(t.(xml.Comment))}
				return n, NoRoot
			}
			cm := make(map[string]interface{}, 2)
			cm[textK] = string(t.(xml.Comment))
			cm[seqK] = seq
			seq++
			na[commentK] = cm
		case xml.Directive:
			if n == nil { // no root 'key'
				n = map[string]interface{}{directiveK: string(t.(xml.Directive))}
				return n, NoRoot
			}
			dm := make(map[string]interface{}, 2)
			dm[textK] = string(t.(xml.Directive))
			dm[seqK] = seq
			seq++
			na[directiveK] = dm
		case xml.ProcInst:
			if n == nil {
				na = map[string]interface{}{targetK: t.(xml.ProcInst).Target, instK: string(t.(xml.ProcInst).Inst)}
				n = map[string]interface{}{procinstK: na}
				return n, NoRoot
			}
			pm := make(map[string]interface{}, 3)
			pm[targetK] = t.(xml.ProcInst).Target
			pm[instK] = string(t.(xml.ProcInst).Inst)
			pm[seqK] = seq
			seq++
			na[procinstK] = pm
		default:
			// noop - shouldn't ever get here, now, since we handle all token types
		}
	}
}

// ------------------ END: NewMapXml & NewMapXmlReader -------------------------

// --------------------- mv.XmlSeq & mv.XmlSeqWriter -------------------------

// Xml encodes a MapSeq as XML with elements sorted on #seq.  The companion of NewMapXmlSeq().
// The following rules apply.
//    - The "#seq" key value is used to seqence the subelements or attributes only.
//    - The "#attr" map key identifies the map of attribute map[string]interface{} values with "#text" key.
//    - The "#comment" map key identifies a comment in the value "#text" map entry - <!--comment-->.
//    - The "#directive" map key identifies a directive in the value "#text" map entry - <!directive>.
//    - The "#procinst" map key identifies a process instruction in the value "#target" and "#inst"
//      map entries - <?target inst?>.
//    - Value type encoding:
//          > string, bool, float64, int, int32, int64, float32: per "%v" formating
//          > []bool, []uint8: by casting to string
//          > structures, etc.: handed to xml.Marshal() - if there is an error, the element
//            value is "UNKNOWN"
//    - Elements with only attribute values or are null are terminated using "/>" unless XmlGoEmptyElemSystax() called.
//    - If len(mv) == 1 and no rootTag is provided, then the map key is used as the root tag, possible.
//      Thus, `{ "key":"value" }` encodes as "<key>value</key>".
func (mv MapSeq) Xml(rootTag ...string) ([]byte, error) {
	m := map[string]interface{}(mv)
	var err error
	s := new(string)
	p := new(pretty) // just a stub

	if len(m) == 1 && len(rootTag) == 0 {
		for key, value := range m {
			// if it's an array, see if all values are map[string]interface{}
			// we force a new root tag if we'll end up with no key:value in the list
			// so: key:[string_val, bool:true] --> <doc><key>string_val</key><bool>true</bool></doc>
			switch value.(type) {
			case []interface{}:
				for _, v := range value.([]interface{}) {
					switch v.(type) {
					case map[string]interface{}: // noop
					default: // anything else
						err = mapToXmlSeqIndent(false, s, DefaultRootTag, m, p)
						goto done
					}
				}
			}
			err = mapToXmlSeqIndent(false, s, key, value, p)
		}
	} else if len(rootTag) == 1 {
		err = mapToXmlSeqIndent(false, s, rootTag[0], m, p)
	} else {
		err = mapToXmlSeqIndent(false, s, DefaultRootTag, m, p)
	}
done:
	if xmlCheckIsValid {
		d := xml.NewDecoder(bytes.NewReader([]byte(*s)))
		for {
			_, err = d.Token()
			if err == io.EOF {
				err = nil
				break
			} else if err != nil {
				return nil, err
			}
		}
	}
	return []byte(*s), err
}

// The following implementation is provided only for symmetry with NewMapXmlReader[Raw]
// The names will also provide a key for the number of return arguments.

// XmlWriter Writes the MapSeq value as  XML on the Writer.
// See MapSeq.Xml() for encoding rules.
func (mv MapSeq) XmlWriter(xmlWriter io.Writer, rootTag ...string) error {
	x, err := mv.Xml(rootTag...)
	if err != nil {
		return err
	}

	_, err = xmlWriter.Write(x)
	return err
}

// XmlWriteRaw writes the MapSeq value as XML on the Writer. []byte is the raw XML that was written.
// See Map.XmlSeq() for encoding rules.
/*
func (mv MapSeq) XmlWriterRaw(xmlWriter io.Writer, rootTag ...string) ([]byte, error) {
	x, err := mv.Xml(rootTag...)
	if err != nil {
		return x, err
	}

	_, err = xmlWriter.Write(x)
	return x, err
}
*/

// XmlIndentWriter writes the MapSeq value as pretty XML on the Writer.
// See MapSeq.Xml() for encoding rules.
func (mv MapSeq) XmlIndentWriter(xmlWriter io.Writer, prefix, indent string, rootTag ...string) error {
	x, err := mv.XmlIndent(prefix, indent, rootTag...)
	if err != nil {
		return err
	}

	_, err = xmlWriter.Write(x)
	return err
}

// XmlIndentWriterRaw writes the Map as pretty XML on the Writer. []byte is the raw XML that was written.
// See Map.XmlSeq() for encoding rules.
/*
func (mv MapSeq) XmlIndentWriterRaw(xmlWriter io.Writer, prefix, indent string, rootTag ...string) ([]byte, error) {
	x, err := mv.XmlSeqIndent(prefix, indent, rootTag...)
	if err != nil {
		return x, err
	}

	_, err = xmlWriter.Write(x)
	return x, err
}
*/

// -------------------- END: mv.Xml & mv.XmlWriter -------------------------------

// ---------------------- XmlSeqIndent ----------------------------

// XmlIndent encodes a map[string]interface{} as a pretty XML string.
// See MapSeq.XmlSeq() for encoding rules.
func (mv MapSeq) XmlIndent(prefix, indent string, rootTag ...string) ([]byte, error) {
	m := map[string]interface{}(mv)

	var err error
	s := new(string)
	p := new(pretty)
	p.indent = indent
	p.padding = prefix

	if len(m) == 1 && len(rootTag) == 0 {
		// this can extract the key for the single map element
		// use it if it isn't a key for a list
		for key, value := range m {
			if _, ok := value.([]interface{}); ok {
				err = mapToXmlSeqIndent(true, s, DefaultRootTag, m, p)
			} else {
				err = mapToXmlSeqIndent(true, s, key, value, p)
			}
		}
	} else if len(rootTag) == 1 {
		err = mapToXmlSeqIndent(true, s, rootTag[0], m, p)
	} else {
		err = mapToXmlSeqIndent(true, s, DefaultRootTag, m, p)
	}
	if xmlCheckIsValid {
		if _, err = NewMapXml([]byte(*s)); err != nil {
			return nil, err
		}
		d := xml.NewDecoder(bytes.NewReader([]byte(*s)))
		for {
			_, err = d.Token()
			if err == io.EOF {
				err = nil
				break
			} else if err != nil {
				return nil, err
			}
		}
	}
	return []byte(*s), err
}

// where the work actually happens
// returns an error if an attribute is not atomic
func mapToXmlSeqIndent(doIndent bool, s *string, key string, value interface{}, pp *pretty) error {
	var endTag bool
	var isSimple bool
	var noEndTag bool
	var elen int
	var ss string
	p := &pretty{pp.indent, pp.cnt, pp.padding, pp.mapDepth, pp.start}

	switch value.(type) {
	case map[string]interface{}, []byte, string, float64, bool, int, int32, int64, float32:
		if doIndent {
			*s += p.padding
		}
		if key != commentK && key != directiveK && key != procinstK {
			*s += `<` + key
		}
	}
	switch value.(type) {
	case map[string]interface{}:
		val := value.(map[string]interface{})

		if key == commentK {
			*s += `<!--` + val[textK].(string) + `-->`
			noEndTag = true
			break
		}

		if key == directiveK {
			*s += `<!` + val[textK].(string) + `>`
			noEndTag = true
			break
		}

		if key == procinstK {
			*s += `<?` + val[targetK].(string) + ` ` + val[instK].(string) + `?>`
			noEndTag = true
			break
		}

		haveAttrs := false
		// process attributes first
		if v, ok := val[attrK].(map[string]interface{}); ok {
			// First, unroll the map[string]interface{} into a []keyval array.
			// Then sequence it.
			kv := make([]keyval, len(v))
			n := 0
			for ak, av := range v {
				kv[n] = keyval{ak, av}
				n++
			}
			sort.Sort(elemListSeq(kv))
			// Now encode the attributes in original decoding sequence, using keyval array.
			for _, a := range kv {
				vv := a.v.(map[string]interface{})
				switch vv[textK].(type) {
				case string:
					if xmlEscapeChars {
						ss = escapeChars(vv[textK].(string))
					} else {
						ss = vv[textK].(string)
					}
					*s += ` ` + a.k + `="` + ss + `"`
				case float64, bool, int, int32, int64, float32:
					*s += ` ` + a.k + `="` + fmt.Sprintf("%v", vv[textK]) + `"`
				case []byte:
					if xmlEscapeChars {
						ss = escapeChars(string(vv[textK].([]byte)))
					} else {
						ss = string(vv[textK].([]byte))
					}
					*s += ` ` + a.k + `="` + ss + `"`
				default:
					return fmt.Errorf("invalid attribute value for: %s", a.k)
				}
			}
			haveAttrs = true
		}

		// simple element?
		// every map value has, at least, "#seq" and, perhaps, "#text" and/or "#attr"
		_, seqOK := val[seqK] // have key
		if v, ok := val[textK]; ok && ((len(val) == 3 && haveAttrs) || (len(val) == 2 && !haveAttrs)) && seqOK {
			if stmp, ok := v.(string); ok && stmp != "" {
				if xmlEscapeChars {
					stmp = escapeChars(stmp)
				}
				*s += ">" + stmp
				endTag = true
				elen = 1
			}
			isSimple = true
			break
		} else if !ok && ((len(val) == 2 && haveAttrs) || (len(val) == 1 && !haveAttrs)) && seqOK {
			// here no #text but have #seq or #seq+#attr
			endTag = false
			break
		}

		// we now need to sequence everything except attributes
		// 'kv' will hold everything that needs to be written
		kv := make([]keyval, 0)
		for k, v := range val {
			if k == attrK { // already processed
				continue
			}
			if k == seqK { // ignore - just for sorting
				continue
			}
			switch v.(type) {
			case []interface{}:
				// unwind the array as separate entries
				for _, vv := range v.([]interface{}) {
					kv = append(kv, keyval{k, vv})
				}
			default:
				kv = append(kv, keyval{k, v})
			}
		}

		// close tag with possible attributes
		*s += ">"
		if doIndent {
			*s += "\n"
		}
		// something more complex
		p.mapDepth++
		sort.Sort(elemListSeq(kv))
		i := 0
		for _, v := range kv {
			switch v.v.(type) {
			case []interface{}:
			default:
				if i == 0 && doIndent {
					p.Indent()
				}
			}
			i++
			if err := mapToXmlSeqIndent(doIndent, s, v.k, v.v, p); err != nil {
				return err
			}
			switch v.v.(type) {
			case []interface{}: // handled in []interface{} case
			default:
				if doIndent {
					p.Outdent()
				}
			}
			i--
		}
		p.mapDepth--
		endTag = true
		elen = 1 // we do have some content other than attrs
	case []interface{}:
		for _, v := range value.([]interface{}) {
			if doIndent {
				p.Indent()
			}
			if err := mapToXmlSeqIndent(doIndent, s, key, v, p); err != nil {
				return err
			}
			if doIndent {
				p.Outdent()
			}
		}
		return nil
	case nil:
		// terminate the tag
		if doIndent {
			*s += p.padding
		}
		*s += "<" + key
		endTag, isSimple = true, true
		break
	default: // handle anything - even goofy stuff
		elen = 0
		switch value.(type) {
		case string:
			if xmlEscapeChars {
				ss = escapeChars(value.(string))
			} else {
				ss = value.(string)
			}
			elen = len(ss)
			if elen > 0 {
				*s += ">" + ss
			}
		case float64, bool, int, int32, int64, float32:
			v := fmt.Sprintf("%v", value)
			elen = len(v)
			if elen > 0 {
				*s += ">" + v
			}
		case []byte: // NOTE: byte is just an alias for uint8
			// similar to how xml.Marshal handles []byte structure members
			if xmlEscapeChars {
				ss = escapeChars(string(value.([]byte)))
			} else {
				ss = string(value.([]byte))
			}
			elen = len(ss)
			if elen > 0 {
				*s += ">" + ss
			}
		default:
			var v []byte
			var err error
			if doIndent {
				v, err = xml.MarshalIndent(value, p.padding, p.indent)
			} else {
				v, err = xml.Marshal(value)
			}
			if err != nil {
				*s += ">UNKNOWN"
			} else {
				elen = len(v)
				if elen > 0 {
					*s += string(v)
				}
			}
		}
		isSimple = true
		endTag = true
	}
	if endTag && !noEndTag {
		if doIndent {
			if !isSimple {
				*s += p.padding
			}
		}
		switch value.(type) {
		case map[string]interface{}, []byte, string, float64, bool, int, int32, int64, float32:
			if elen > 0 || useGoXmlEmptyElemSyntax {
				if elen == 0 {
					*s += ">"
				}
				*s += `</` + key + ">"
			} else {
				*s += `/>`
			}
		}
	} else if !noEndTag {
		if useGoXmlEmptyElemSyntax {
			*s += `</` + key + ">"
			// *s += "></" + key + ">"
		} else {
			*s += "/>"
		}
	}
	if doIndent {
		if p.cnt > p.start {
			*s += "\n"
		}
		p.Outdent()
	}

	return nil
}

// the element sort implementation

type keyval struct {
	k string
	v interface{}
}
type elemListSeq []keyval

func (e elemListSeq) Len() int {
	return len(e)
}

func (e elemListSeq) Swap(i, j int) {
	e[i], e[j] = e[j], e[i]
}

func (e elemListSeq) Less(i, j int) bool {
	var iseq, jseq int
	var fiseq, fjseq float64
	var ok bool
	if iseq, ok = e[i].v.(map[string]interface{})[seqK].(int); !ok {
		if fiseq, ok = e[i].v.(map[string]interface{})[seqK].(float64); ok {
			iseq = int(fiseq)
		} else {
			iseq = 9999999
		}
	}

	if jseq, ok = e[j].v.(map[string]interface{})[seqK].(int); !ok {
		if fjseq, ok = e[j].v.(map[string]interface{})[seqK].(float64); ok {
			jseq = int(fjseq)
		} else {
			jseq = 9999999
		}
	}

	return iseq <= jseq
}

// =============== https://groups.google.com/forum/#!topic/golang-nuts/lHPOHD-8qio

// BeautifyXml (re)formats an XML doc similar to Map.XmlIndent().
// It preserves comments, directives and process instructions,
func BeautifyXml(b []byte, prefix, indent string) ([]byte, error) {
	x, err := NewMapXmlSeq(b)
	if err != nil {
		return nil, err
	}
	return x.XmlIndent(prefix, indent)
}
