package service

import "fmt"

// RuntimeFactory 根据会话模式选择运行时实现。
// 它不负责创建具体实现，也不读取数据库，便于测试和替换。
type RuntimeFactory struct {
	Legacy ChatRuntime
	Story  ChatRuntime
}

func (f RuntimeFactory) Resolve(storyEnabled bool) (ChatRuntime, error) {
	if storyEnabled {
		if f.Story == nil {
			return nil, fmt.Errorf("story chat runtime is not configured")
		}
		return f.Story, nil
	}
	if f.Legacy == nil {
		return nil, fmt.Errorf("legacy chat runtime is not configured")
	}
	return f.Legacy, nil
}
