package cname_flattening

import (
	"fmt"
	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/forward"
	"github.com/coredns/coredns/plugin/pkg/parse"
	"github.com/coredns/coredns/plugin/pkg/proxy"
	"github.com/coredns/coredns/plugin/pkg/transport"
	"github.com/pkg/errors"
	"strconv"
	"strings"
	"time"
)

const defaultExpire = 10 * time.Second

// init registers this plugin.
func init() { plugin.Register("cname_flattening", setup) }

// setup is the function that gets called when the config parser see the token "example". Setup is responsible
// for parsing any extra options the example plugin may have. The first token this function sees is "example".
func setup(c *caddy.Controller) error {
	cname := CName{}
	for c.Next() {
		// First parameter is the depth of the CNAME chain to follow
		args := c.RemainingArgs()
		fmt.Println(args)
		if len(args) <= 3 {
			return plugin.Error("cname_flattening", c.ArgErr())
		}
		if strings.EqualFold("max_depth", args[0]) {
			maxDepth, err := strconv.Atoi(args[1])
			if err != nil {
				return plugin.Error("cname_flattening", err)
			}
			cname.MaxDepth = maxDepth
			// Rest of parameters are the forward plugin settings
			forwardHandler, err := initForward(c, args[2:])
			if err != nil {
				return plugin.Error("cname_flattening", errors.Wrap(err, "failed to initialize forward plugin"))
			}
			cname.Forward = forwardHandler
			fmt.Println("Forward plugin settings: ", args[2:])
			fmt.Println("Handler: ", forwardHandler.List())
		} else {
			return fmt.Errorf("unsupported parameter %s for upstream setting", args[0])
		}
	}

	c.Next() // Ignore "example" and give us the next token.
	if c.NextArg() {
		// If there was another token, return an error, because we don't have any configuration.
		// Any errors returned from this setup function should be wrapped with plugin.Error, so we
		// can present a slightly nicer error message to the user.
		return plugin.Error("example", c.ArgErr())
	}
	// Add the Plugin to CoreDNS, so Servers can use it in their plugin chain.
	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		cname.Next = next
		return cname
	})

	// All OK, return a nil error.
	return nil
}

func initForward(c *caddy.Controller, parameters []string) (*forward.Forward, error) {
	f := forward.New()

	if len(parameters) == 0 {
		return f, c.ArgErr()
	}

	toHosts, err := parse.HostPortOrFile(parameters...)
	if err != nil {
		return f, err
	}

	for c.NextBlock() {
		return f, fmt.Errorf("additional parameters not allowed")
	}

	for _, host := range toHosts {
		trans, h := parse.Transport(host)
		if trans != transport.DNS {
			return f, fmt.Errorf("only dns transport allowed")
		}
		p := proxy.NewProxy("alternate", h, trans)
		p.SetExpire(defaultExpire)
		f.SetProxy(p)
	}

	return f, nil
}
