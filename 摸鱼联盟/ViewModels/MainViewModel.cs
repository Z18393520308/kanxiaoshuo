using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Linq;
using System.Text;
using System.Threading.Tasks;
using System.Windows.Forms;
using System.Windows.Media;
using 摸鱼联盟.tools;

namespace 摸鱼联盟.ViewModels
{
    /// <summary>
    /// 功能：<对此文件主要功能的描述>
    /// 作者：zhangsheng
    /// 创建日期：2022/12/30 10:54:15
    /// </summary>
    public class MainViewModel: NotifyBase
    {
        #region <属性>

        /// <summary>
        /// 
        /// </summary>
        private int fontSize=5;
        public int FontSize
        {
            get { return fontSize; }
            set
            { 
                fontSize = value;
                DoNotify("FontSize");
            }
        }


        private Brush foreground=(Brush)new BrushConverter().ConvertFromString("#939393");

        public Brush Foreground
        {
            get { return foreground; }
            set 
            {
                foreground = value;
                DoNotify("foreground");
            }
        }



        private string fileNmae;

        public string FileNmae
        {
            get { return fileNmae; }
            set { fileNmae = value; }
        }


        public CommandBase btn_Color1 { set; get; }

        public CommandBase btn_Color2 { set; get; }

        public CommandBase btn_Color3 { set; get; }

        public CommandBase btn_Color4 { set; get; }

        public CommandBase btn_Color5 { set; get; }

        public CommandBase btn_Color_Select { set; get; }

        public CommandBase btn_Path_Select { set; get; }

        #endregion <属性>

        #region <构造方法>


        public MainViewModel()
        {
            btn_Path_Select = new CommandBase();
            btn_Path_Select.DoCExecute = new Action<object>((o) => { PathSelect(o); });
            btn_Path_Select.DoCanExecute = new Func<object, bool>((o) => { return true; });


            btn_Color1 = new CommandBase();
            btn_Color1.DoCExecute = new Action<object>((o) => { SettingWhite(o); });
            btn_Color1.DoCanExecute = new Func<object, bool>((o) => { return true; });

            btn_Color2 = new CommandBase();
            btn_Color2.DoCExecute = new Action<object>((o) => { SettingBlack(o); });
            btn_Color2.DoCanExecute = new Func<object, bool>((o) => { return true; });

            btn_Color3 = new CommandBase();
            btn_Color3.DoCExecute = new Action<object>((o) => { SettingYellow(o); });
            btn_Color3.DoCanExecute = new Func<object, bool>((o) => { return true; });

            btn_Color4 = new CommandBase();
            btn_Color4.DoCExecute = new Action<object>((o) => { SettingBlue(o); });
            btn_Color4.DoCanExecute = new Func<object, bool>((o) => { return true; });

            btn_Color5 = new CommandBase();
            btn_Color5.DoCExecute = new Action<object>((o) => { SettingGreen(o); });
            btn_Color5.DoCanExecute = new Func<object, bool>((o) => { return true; });


            FileNmae = ConfigHelper.ContentValue("BOOKRACK", "Reading", System.AppDomain.CurrentDomain.BaseDirectory + "Config.ini");
        }
        #endregion <构造方法>

        #region <方法>


        public void SettingWhite(Object O)
        {
            Foreground= (Brush)new BrushConverter().ConvertFromString("#FFFFFF");
        }

        public void SettingBlack(Object O)
        {
            Foreground = (Brush)new BrushConverter().ConvertFromString("#000000");
        }

        public void SettingYellow(Object O)
        {
            Foreground = (Brush)new BrushConverter().ConvertFromString("#9DE5FE");
        }

        public void SettingBlue(Object O)
        {
            Foreground = (Brush)new BrushConverter().ConvertFromString("#FCEABA");
        }

        public void SettingGreen(Object O)
        {
            Foreground = (Brush)new BrushConverter().ConvertFromString("#D0EABB");
        }

        public void PathSelect(Object O)
        {
            OpenFileDialog dialog = new OpenFileDialog();
            dialog.Multiselect = true;//该值确定是否可以选择多个文件
            dialog.Title = "请选择文件夹";
            dialog.Filter = "文本文件(*.txt)|*.txt";
            if (dialog.ShowDialog() == System.Windows.Forms.DialogResult.OK)
            {
                string file = dialog.FileName;
                FileNmae = file;
                ConfigHelper.WritePrivateProfileString("BOOKRACK", "Reading", file, System.AppDomain.CurrentDomain.BaseDirectory + "Config.ini");
            }
        }

        #endregion <方法>

        #region <事件>
        #endregion <事件>
    }
}
