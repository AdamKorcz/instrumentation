package utils


type TextRewriter struct {
	FilePath     string
	FileContents []byte
	ReplaceFrom  string
	ReplaceTo    string
	StartOffset  int
	EndOffset    int
}