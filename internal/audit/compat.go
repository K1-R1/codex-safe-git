package audit

func (l Logger) WriteAuditCompat(action, repo string, err error) {
	reason := ""
	if err != nil {
		reason = err.Error()
	}
	l.Write(Entry{Action: action, Result: "refused", Repo: repo, Reason: reason})
}
