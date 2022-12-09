using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Data;
using System.Windows.Documents;
using System.Windows.Forms;
using System.Windows.Input;
using System.Windows.Media;
using System.Windows.Media.Imaging;
using System.Windows.Navigation;
using System.Windows.Shapes;
using 摸鱼联盟.tools;
using static System.Windows.Forms.VisualStyles.VisualStyleElement.Window;

namespace 摸鱼联盟
{
    /// <summary>
    /// MainWindow.xaml 的交互逻辑
    /// </summary>
    public partial class MainWindow : Window
    {
        public MainWindow()
        {
            InitializeComponent();
        }

        private void Button_Click(object sender, RoutedEventArgs e)
        {
            this.Hide();
            looktxt looktxt=new looktxt();
            looktxt.Show();
        }

        private void Window_Loaded(object sender, RoutedEventArgs e)
        {
            txt_Reading.Text = ConfigHelper.ContentValue("BOOKRACK", "Reading", System.AppDomain.CurrentDomain.BaseDirectory + "Config.ini");
        }

        private void Button_Click_1(object sender, RoutedEventArgs e)
        {
            OpenFileDialog dialog = new OpenFileDialog();
            dialog.Multiselect = true;//该值确定是否可以选择多个文件
            dialog.Title = "请选择文件夹";
            dialog.Filter = "文本文件(*.txt)|*.txt";
            if (dialog.ShowDialog() == System.Windows.Forms.DialogResult.OK)
            {
                string file = dialog.FileName;
                txt_Reading.Text = file;
                ConfigHelper.WritePrivateProfileString("BOOKRACK", "Reading", file, System.AppDomain.CurrentDomain.BaseDirectory + "Config.ini");
            }
        }

        private void TextBox_TextChanged(object sender, TextChangedEventArgs e)
        {

        }
    }
}
