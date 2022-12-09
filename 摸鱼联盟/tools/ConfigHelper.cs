using System;
using System.Collections.Generic;
using System.Linq;
using System.Runtime.InteropServices;
using System.Text;
using System.Threading.Tasks;

namespace 摸鱼联盟.tools
{
    /// <summary>
    /// 功能：<对此文件主要功能的描述>
    /// 作者：zhangsheng
    /// 创建日期：2022/12/9 10:46:54
    /// </summary>
    public class ConfigHelper
    {
        #region  API函数声明
        [DllImport("kernel32")]
        public static extern long WritePrivateProfileString(string section, string key, string val, string filepath);
        [DllImport("kernel32")]
        public static extern long GetPrivateProfileString(string section, string key, string def, StringBuilder retval, int size, string filepath);
        #endregion
        public static string ContentValue(string Section, string key, string strFilePath)
        {
            StringBuilder temp = new StringBuilder(1024);
            GetPrivateProfileString(Section, key, "", temp, 1024, strFilePath);
            return temp.ToString();
        }
    }
}
