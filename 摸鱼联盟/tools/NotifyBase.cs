using System;
using System.Collections.Generic;
using System.ComponentModel;
using System.Linq;
using System.Runtime.CompilerServices;
using System.Text;
using System.Threading.Tasks;
using System.Windows;

namespace 摸鱼联盟.tools
{
    /// <summary>
    /// 功能：<对此文件主要功能的描述>
    /// 作者：zhangsheng
    /// 创建日期：2022/12/30 10:51:20
    /// </summary>
    public class NotifyBase : INotifyPropertyChanged
    {

        public event PropertyChangedEventHandler PropertyChanged;

        public void DoNotify([CallerMemberName] string propName = "")
        {
            PropertyChanged?.Invoke(this, new PropertyChangedEventArgs(propName));
        }

        public void DoNotify1([CallerMemberName] string propName = "")
        {
            Application.Current.Dispatcher.Invoke(new Action(delegate
            {
                if (PropertyChanged != null)
                {
                    PropertyChanged(this, new PropertyChangedEventArgs(propName));
                }

            }));

        }
    }
}
