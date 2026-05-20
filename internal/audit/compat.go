package audit

func (l Logger) WriteAuditCompat(action, repo string, err error) {
	_ = l.WriteAuditCompatChecked(action, repo, err)
}

func (l Logger) WriteAuditCompatChecked(action, repo string, err error) error {
	reason := ""
	if err != nil {
		reason = err.Error()
	}
	return l.WriteChecked(Entry{Action: action, Result: "refused", Repo: repo, Reason: reason})
}
