package game

type MessageRewriter interface {
	Rewrite(message string, style Style) (string, error)
}

type localMessageRewriter struct{}

func (localMessageRewriter) Rewrite(message string, style Style) (string, error) {
	return RewriteMessage(message, style), nil
}

type ServiceOption func(*Service)

func WithMessageRewriter(rewriter MessageRewriter) ServiceOption {
	return func(service *Service) {
		if rewriter != nil {
			service.messageRewriter = rewriter
		}
	}
}
