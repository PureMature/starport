// Package email provides a Starlark module that sends email using Resend API.
package email

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"starport/base"

	"github.com/1set/gut/ystring"
	"github.com/1set/starlet"
	"github.com/1set/starlet/dataconv"
	"github.com/1set/starlet/dataconv/types"
	"github.com/resend/resend-go/v2"
	"github.com/samber/lo"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	renderer "github.com/yuin/goldmark/renderer/html"
	"go.starlark.net/starlark"
)

// ModuleName defines the expected name for this module when used in Starlark's load() function, e.g., load('email', 'send')
const ModuleName = "email"

// EmailModule wraps the BaseModule with specific functionality for sending emails.
type EmailModule struct {
	baseModule *base.BaseModule[string]
}

// NewModule creates a new instance of EmailModule.
func NewModule() *EmailModule {
	bm := base.NewBaseModule[string]()
	return &EmailModule{baseModule: bm}
}

// NewModuleWithConfig creates a new instance of EmailModule with the given configuration values.
func NewModuleWithConfig(resendAPIKey, senderDomain string) *EmailModule {
	bm := base.NewBaseModule[string]()
	bm.SetConfig("resend_api_key", func() string { return resendAPIKey })
	bm.SetConfig("sender_domain", func() string { return senderDomain })
	return &EmailModule{baseModule: bm}
}

// NewModuleWithGetter creates a new instance of EmailModule with the given configuration getters.
func NewModuleWithGetter(resendAPIKey, senderDomain base.ConfigGetter[string]) *EmailModule {
	bm := base.NewBaseModule[string]()
	bm.SetConfig("resend_api_key", resendAPIKey)
	bm.SetConfig("sender_domain", senderDomain)
	return &EmailModule{baseModule: bm}
}

// LoadModule returns the Starlark module loader with the email-specific functions.
func (m *EmailModule) LoadModule() starlet.ModuleLoader {
	additionalFuncs := starlark.StringDict{
		"send": m.genSendFunc(),
	}
	return m.baseModule.LoadModule(ModuleName, additionalFuncs)
}

// genSendFunc generates the Starlark callable function to send an email.
func (m *EmailModule) genSendFunc() starlark.Callable {
	return starlark.NewBuiltin(ModuleName+".send", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		// Load config: resend_api_key is required, sender_domain is optional
		resendAPIKey, err := m.baseModule.GetConfig("resend_api_key")
		if err != nil {
			return starlark.None, fmt.Errorf("resend_api_key is not set")
		}
		senderDomain, _ := m.baseModule.GetConfig("sender_domain")

		// parse args
		newOneOrListStr := func() *types.OneOrMany[starlark.String] { return types.NewOneOrManyNoDefault[starlark.String]() }
		var (
			subject            types.StringOrBytes         // must be set
			bodyHTML           types.NullableStringOrBytes // one of the three must be set
			bodyText           types.NullableStringOrBytes
			bodyMarkdown       types.NullableStringOrBytes
			toAddresses        = newOneOrListStr() // must be set
			ccAddresses        = newOneOrListStr()
			bccAddresses       = newOneOrListStr()
			fromAddress        types.StringOrBytes // one of the two must be set
			fromNameID         types.StringOrBytes
			replyAddress       types.StringOrBytes // two of them are optional
			replyNameID        types.StringOrBytes
			attachmentFiles    = newOneOrListStr()
			attachmentContents = types.NewOneOrManyNoDefault[*starlark.Dict]()
		)
		if err := starlark.UnpackArgs(b.Name(), args, kwargs,
			"subject", &subject,
			"body_html?", &bodyHTML, "body_text?", &bodyText, "body_markdown?", &bodyMarkdown,
			"to", toAddresses, "cc?", ccAddresses, "bcc?", bccAddresses,
			"from?", &fromAddress, "from_id?", &fromNameID,
			"reply_to?", &replyAddress, "reply_id?", &replyNameID,
			"attachment_files?", attachmentFiles, "attachment?", attachmentContents); err != nil {
			return starlark.None, err
		}

		// validate args
		if body := []string{bodyHTML.GoString(), bodyText.GoString(), bodyMarkdown.GoString()}; lo.EveryBy(body, ystring.IsBlank) {
			return starlark.None, fmt.Errorf("one of body_html, body_text, or body_markdown must be non-blank")
		}
		if toAddresses.Len() == 0 {
			return starlark.None, fmt.Errorf("to must be set and non-empty")
		}
		if from := []string{fromAddress.GoString(), fromNameID.GoString()}; lo.EveryBy(from, ystring.IsBlank) {
			return starlark.None, fmt.Errorf("one of from or from_id must be non-blank")
		}

		// convert from to send address
		var sendAddr string
		if fromAddr := fromAddress.GoString(); ystring.IsNotBlank(fromAddr) {
			sendAddr = fromAddr
		} else if fromID := fromNameID.GoString(); ystring.IsNotBlank(fromID) {
			if ystring.IsNotBlank(senderDomain) {
				sendAddr = fromID + "@" + senderDomain
			} else {
				return starlark.None, fmt.Errorf("sender_domain should be set when from_id is used")
			}
		} else {
			return starlark.None, fmt.Errorf("no valid from or from_id found")
		}

		// prepare request
		convGoString := func(v []starlark.String) []string {
			l := make([]string, len(v))
			for i, vv := range v {
				l[i] = dataconv.StarString(vv)
			}
			return l
		}
		req := &resend.SendEmailRequest{
			From:    sendAddr,
			To:      convGoString(toAddresses.Slice()),
			Cc:      convGoString(ccAddresses.Slice()),
			Bcc:     convGoString(bccAddresses.Slice()),
			Subject: subject.GoString(),
		}

		// for body content
		if !bodyHTML.IsNullOrEmpty() {
			// directly use HTML content
			req.Html = bodyHTML.GoString()
		} else if !bodyText.IsNullOrEmpty() {
			// directly use text content
			req.Text = bodyText.GoString()
		} else if !bodyMarkdown.IsNullOrEmpty() {
			// convert markdown to HTML
			markdown := goldmark.New(
				goldmark.WithRendererOptions(
					renderer.WithUnsafe(),
				),
				goldmark.WithExtensions(
					extension.Strikethrough,
					extension.Table,
					extension.Linkify,
				),
			)
			html := bytes.NewBufferString("")
			_ = markdown.Convert([]byte(bodyMarkdown.GoString()), html)
			req.Html = html.String()
		}

		// for attachments
		if fps := attachmentFiles.Slice(); len(fps) > 0 {
			// load file content and attach
			for _, r := range fps {
				fp := r.GoString()
				c, err := ioutil.ReadFile(fp)
				if err != nil {
					return starlark.None, err
				}
				n := filepath.Base(fp)
				req.Attachments = append(req.Attachments, &resend.Attachment{
					Filename: n,
					Content:  c,
				})
			}
		}
		if dcts := attachmentContents.Slice(); len(dcts) > 0 {
			// convert dict to attachment and attach
			for _, r := range dcts {
				fn, ok, err := r.Get(starlark.String("name"))
				if !ok || err != nil {
					return starlark.None, fmt.Errorf("attachment must have a name")
				}
				ct, ok, err := r.Get(starlark.String("content"))
				if !ok || err != nil {
					return starlark.None, fmt.Errorf("attachment must have content")
				}
				req.Attachments = append(req.Attachments, &resend.Attachment{
					Filename: dataconv.StarString(fn),
					Content:  []byte(dataconv.StarString(ct)),
				})
			}
		}

		// send it
		client := resend.NewClient(resendAPIKey)
		sent, err := client.Emails.Send(req)
		if err != nil {
			return starlark.None, err
		}
		return starlark.String(sent.Id), nil
	})
}
