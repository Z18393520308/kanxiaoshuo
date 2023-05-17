using System;
using System.Activities;
using System.Collections.Generic;
using System.Diagnostics;
using System.Linq;
using System.Text;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Data;
using System.Windows.Documents;

using System.Windows.Input;
using System.Windows.Media;
using System.Windows.Media.Imaging;
using System.Windows.Shapes;
using 摸鱼联盟.tools;

namespace 摸鱼联盟
{
    /// <summary>
    /// looktxt.xaml 的交互逻辑
    /// </summary>
    public partial class looktxt : Window
    {
        public looktxt()
        {
            InitializeComponent();
        }
        Point _pressedPosition;
        bool _isDragMoved = false;

        private void Window_PreviewMouseLeftButtonDown(object sender, MouseButtonEventArgs e)
        {
            if (_isDragMoved)
            {
                _isDragMoved = false;
                e.Handled = true;
            }
        }

        private void Window_PreviewMouseMove(object sender, MouseEventArgs e)
        {
            if (Mouse.LeftButton == MouseButtonState.Pressed && _pressedPosition != e.GetPosition(this))
            {
                _isDragMoved = true;
                DragMove();

                ConfigHelper.WritePrivateProfileString("WINPosition", "Left", this.Left.ToString(), System.AppDomain.CurrentDomain.BaseDirectory + "Config.ini");
                ConfigHelper.WritePrivateProfileString("WINPosition", "Top", this.Top.ToString(), System.AppDomain.CurrentDomain.BaseDirectory + "Config.ini");
            }

        }

        private void Window_PreviewMouseLeftButtonUp(object sender, MouseButtonEventArgs e)
        {
            _pressedPosition = e.GetPosition(this);
        }

        private void Window_Deactivated(object sender, EventArgs e)
        {
            Window window = (Window)sender;
            window.Topmost = true;
        }

        int Lineindex=1;
        Key Keyindex = Key.None;


        private void Window_Loaded(object sender, RoutedEventArgs e)
        {
            Task.Run(async () =>
            {
                while (true)
                {
                    Keyindex = Key.None; ;
                   await Task.Delay(50);
                }
            });


            Hotkey.Regist(this, HotkeyModifiers.MOD_ALT, Key.T, HotKeyCallBackHanlder);
            Hotkey.Regist(this, HotkeyModifiers.MOD_ALT, Key.S, HotKeyCallBackHanlder);
            Hotkey.Regist(this, HotkeyModifiers.MOD_ALT, Key.C, HotKeyCallBackHanlder);
            Hotkey.Regist(this, HotkeyModifiers.None, Key.Down, HotKeyCallBackHanlder);
            Hotkey.Regist(this, HotkeyModifiers.None, Key.Up, HotKeyCallBackHanlder);

            this.Left = Convert.ToDouble(ConfigHelper.ContentValue("WINPosition", "Left", System.AppDomain.CurrentDomain.BaseDirectory + "Config.ini"));
            this.Top = Convert.ToDouble(ConfigHelper.ContentValue("WINPosition", "Top", System.AppDomain.CurrentDomain.BaseDirectory + "Config.ini"));

            txtlook.Text = Readtxt.OpenFileWS(ConfigHelper.ContentValue("BOOKRACK", "Reading", System.AppDomain.CurrentDomain.BaseDirectory + "Config.ini")); 

            Lineindex= Convert.ToInt32(ConfigHelper.ContentValue("BOOKRACK", "Lineindex", System.AppDomain.CurrentDomain.BaseDirectory + "Config.ini"));
            txtlook.ScrollToLine(Lineindex);
        }

        object obj=new object();
        public void HotKeyCallBackHanlder(HotkeyModifiers hotkeyModifiers,Key key)
        {
            lock (obj)
            {
                if (Keyindex != key)
                {

                    Keyindex = key;
                    Console.WriteLine("22:" + key);
                    switch (key)
                    {
                        case Key.C:
                            Window window = (Window)LOOK;
                            window.Topmost = true;
                            window.Hide();
                            break;
                        case Key.S:
                            Window windowSHOW = (Window)LOOK;
                            windowSHOW.Topmost = true;
                            windowSHOW.Show();
                            break;
                        case Key.T:
                            Window windowT = (Window)LOOK;
                            windowT.Topmost = true;
                            windowT.Left = 100;
                            windowT.Top = 100;
                            break;

                        case Key.Up:
                            Lineindex--;
                            if (Lineindex <= 0)
                            {
                                Lineindex = 0;
                            }
                            txtlook.ScrollToLine(Lineindex);
                            ConfigHelper.WritePrivateProfileString("BOOKRACK", "Lineindex", Lineindex.ToString(), System.AppDomain.CurrentDomain.BaseDirectory + "Config.ini");
                            break;
                        case Key.Down:
                            Lineindex++;
                            if (Lineindex >= txtlook.LineCount)
                            {
                                Lineindex = txtlook.LineCount-1;
                            }

                            ConfigHelper.WritePrivateProfileString("BOOKRACK", "Lineindex", Lineindex.ToString(), System.AppDomain.CurrentDomain.BaseDirectory + "Config.ini");
                            txtlook.ScrollToLine(Lineindex);
                            break;
                        default:
                            break;
                    }
                }







            }

        }

    }
}
