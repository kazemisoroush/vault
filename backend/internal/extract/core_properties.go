package extract

// coreProperties is the docProps/core.xml shape of an office document's built-in metadata.
type coreProperties struct {
	Creator  string `xml:"creator"`
	Title    string `xml:"title"`
	Subject  string `xml:"subject"`
	Created  string `xml:"created"`
	Modified string `xml:"modified"`
}
