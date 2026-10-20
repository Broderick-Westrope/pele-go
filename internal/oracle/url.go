package oracle

import (
	"fmt"

	"github.com/dop251/goja"

	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
	"github.com/Broderick-Westrope/pele-go/internal/util"
)

// urlClass is the part of URL that sanitizeUrl, its only user, reaches: new URL(s) on an
// http or https URL, setting protocol and hostname to their lower case, and toString(). The
// parser already lower-cases both, so the setters have nothing to do.
const urlClass = `globalThis.URL = class URL {
  constructor(input) {
    const href = __peleParseHTTPURL(String(input));
    if (href === null) throw new TypeError('Invalid URL');
    this.href = href;
  }
  get protocol() { return this.href.slice(0, this.href.indexOf(':') + 1); }
  set protocol(_) {}
  get hostname() { return ''; }
  set hostname(_) {}
  toString() { return this.href; }
};`

// installURL gives goja, which has no URL, one backed by the port's WHATWG parser. Links in
// goja then follow the port's parsing; the V8 oracle still checks it independently.
func installURL(vm *goja.Runtime) error {
	parse := func(call goja.FunctionCall) goja.Value {
		href, ok := util.ParseHTTPURL(goString(call.Argument(0)))
		if !ok {
			return goja.Null()
		}
		return vm.ToValue(goja.StringFromUTF16(jsstr.UTF16(href)))
	}
	if err := vm.Set("__peleParseHTTPURL", parse); err != nil {
		return fmt.Errorf("install URL: %w", err)
	}
	if _, err := vm.RunString(urlClass); err != nil {
		return fmt.Errorf("install URL: %w", err)
	}
	return nil
}
