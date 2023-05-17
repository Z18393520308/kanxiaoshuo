using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using System.Threading.Tasks;
using System.Windows.Input;

namespace 摸鱼联盟.tools
{
    /// <summary>
    /// 功能：<对此文件主要功能的描述>
    /// 作者：zhangsheng
    /// 创建日期：2022/12/30 11:16:59
    /// </summary>
    public class CommandBase : ICommand
    {

        public Action<object> DoCExecute { get; set; }
        public Func<object, bool> DoCanExecute { get; set; }

        public event EventHandler CanExecuteChanged;

        public bool CanExecute(object parameter)
        {
            return DoCanExecute?.Invoke(parameter) == true;
        }

        public void Execute(object parameter)
        {
            DoCExecute?.Invoke(parameter);

        }
    }
}
