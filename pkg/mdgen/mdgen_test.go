package mdgen

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"testing"

	"github.com/bwplotka/mdox/pkg/mdformatter"
	"github.com/bwplotka/mdox/pkg/testutil"
)

func TestEmbed(t *testing.T) {
	file, err := os.OpenFile("testdata/embed_in.md", os.O_RDONLY, 0)
	testutil.Ok(t, err)
	defer file.Close()

	f := mdformatter.New(context.Background(), mdformatter.WithCodeBlockTransformer(&genCodeBlockTransformer{}))

	buf := bytes.Buffer{}
	testutil.Ok(t, f.Format(file, &buf))
	testutil.Ok(t, ioutil.WriteFile("test.md", buf.Bytes(), os.ModePerm))
	//
	//exp, err := ioutil.ReadFile("testdata/embed_in.md")
	//testutil.Ok(t, err)
	//fmt.Println(string(exp))
	//testutil.Equals(t, string(exp), buf.String())
}
