package mxj

// Checks whether the path exists. If err != nil then 'false' is returned
// along with the error encountered parsing either the "path" or "subkeys"
// argument.
func (mv Map) Exists(path string, subkeys ...string) (bool, error) {
	v, err := mv.ValuesForPath(path, subkeys...)
	return (err == nil && len(v) > 0), err
}
