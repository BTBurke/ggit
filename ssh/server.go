package ssh

import (
	"fmt"
	"log"

	"git.icyphox.sh/legit/config"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/git"
	"github.com/charmbracelet/wish/logging"
)

func AuthHandler(c *config.Config) wish.Middleware {
	return func(h ssh.Handler) ssh.Handler {
		return func(s ssh.Session) {
			// handle login link if raw SSH session
			if len(s.Command()) == 0 {
				for _, ident := range c.SSH.Identity {
					k, _, _, _, _ := ssh.ParseAuthorizedKey([]byte(ident.PubKey))
					if ssh.KeysEqual(s.PublicKey(), k) {
						wish.Printf(s, "Hello %s.\n\nLog in using this link:\n%s", ident.Name, "https://test.link\n\n")
						s.Exit(0)
					}
				}
				// no authed identity, close connection
				s.Exit(127)
			}
			h(s)
		}
	}
}

func authPubKey(c *config.Config, k ssh.PublicKey) (bool, string) {
	for _, ident := range c.SSH.Identity {
		k1, _, _, _, _ := ssh.ParseAuthorizedKey([]byte(ident.PubKey))
		if ssh.KeysEqual(k, k1) {
			return true, ident.Name
		}
	}
	return false, ""
}

func NewServer(c *config.Config) *ssh.Server {
	s, err := wish.NewServer(
		wish.WithAddress(fmt.Sprintf("%s:%d", c.SSH.Host, c.SSH.Port)),
		wish.WithMiddleware(
			logging.Middleware(),
			AuthHandler(c),
			git.Middleware(c.Repo.ScanPath, hooks{c}),
		),
	)
	if err != nil {
		log.Fatal("SSH server error: %s", err)
	}
	return s
}

type hooks struct {
	c *config.Config
}

func (h hooks) AuthRepo(repo string, k ssh.PublicKey) git.AccessLevel {
	ok, name := authPubKey(h.c, k)
	if ok {
		log.Printf("Authorized git ssh access for %s", name)
		return git.ReadWriteAccess
	}
	return git.NoAccess
}

func (hooks) Push(repo string, k ssh.PublicKey)  { log.Printf("Push to %s", repo) }
func (hooks) Fetch(repo string, k ssh.PublicKey) { log.Printf("Fetch from %s", repo) }

var _ git.Hooks = hooks{}
