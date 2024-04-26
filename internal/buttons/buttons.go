package buttons

var (
	ApproveButton = New("✔️ Approve")
	SkipButton    = New("👀 Skip")
	RejectButton  = New("❌ Reject")
)

type Button struct {
	text string
}

func New(text string) Button {
	return Button{
		text: text,
	}
}

func (b Button) Text() string {
	return b.text
}
