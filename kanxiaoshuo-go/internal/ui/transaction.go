package ui

import (
	"fmt"

	"kanxiaoshuo-go/internal/config"
)

// readerControls 将操作顺序与 Win32 实现隔开，可在任何平台验证切书失败不会覆盖配置。
// apply 的契约是失败保留原阅读器；close 保存失败时也必须保留窗口。
type readerControls struct {
	start func(config.Settings) error
	apply func(config.Settings) error
	show  func() error
	seek  func(int) error
	close func() error
}

func restoreReader(before config.Settings, wasStarted bool, controls readerControls, cause error) (bool, error) {
	if !wasStarted {
		if err := controls.close(); err != nil {
			// Close 刷盘失败会保留窗口，仍必须让托盘和关闭按钮知道阅读器正在运行。
			return true, fmt.Errorf("%v；关闭新阅读窗口也失败：%v", cause, err)
		}
		return false, cause
	}
	if err := controls.apply(before); err != nil {
		return true, fmt.Errorf("%v；恢复原设置也失败：%v", cause, err)
	}
	if err := controls.seek(before.PositionBytes); err != nil {
		return true, fmt.Errorf("%v；恢复原阅读位置也失败：%v", cause, err)
	}
	return true, cause
}

// commitPreferences 先验证读取及实际热键注册，再保存偏好；任何失败都保留设置入口。
func commitPreferences(before, next config.Settings, wasStarted bool, controls readerControls, persist func(config.Settings) error) (bool, error) {
	var err error
	if wasStarted {
		err = controls.apply(next)
	} else {
		err = controls.start(next)
	}
	if err != nil {
		return wasStarted, err
	}
	if err := controls.show(); err != nil {
		return restoreReader(before, wasStarted, controls, err)
	}
	if err := persist(next); err != nil {
		return restoreReader(before, wasStarted, controls, fmt.Errorf("保存设置失败：%w", err))
	}
	return true, nil
}

// commitRelocation 合并目标为当前书时必须 Seek；普通 Apply 保留当前位置，不能代替显式恢复书签。
func commitRelocation(before, next config.Settings, active bool, controls readerControls, persist func() error) error {
	if active {
		if err := controls.apply(next); err != nil {
			return err
		}
		if err := controls.seek(next.PositionBytes); err != nil {
			_, failure := restoreReader(before, true, controls, err)
			return failure
		}
	}
	if err := persist(); err != nil {
		if active {
			_, failure := restoreReader(before, true, controls, err)
			return failure
		}
		return err
	}
	return nil
}
