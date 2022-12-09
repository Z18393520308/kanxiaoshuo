using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text;
using System.Threading.Tasks;

namespace 摸鱼联盟.tools
{
    /// <summary>
    /// 功能：<对此文件主要功能的描述>
    /// 作者：zhangsheng
    /// 创建日期：2022/12/9 11:06:54
    /// </summary>
    public class Readtxt
    {
        public static string OpenFileWS(string filePath)
        {
            StringBuilder stringBuilder = new StringBuilder();


            FileStream fileStream = new FileStream(filePath, FileMode.Open);
            StreamReader sr = new StreamReader(fileStream);
            string line;
            while ((line = sr.ReadLine()) != null)
            {
                stringBuilder.Append(line);
            }
            return stringBuilder.ToString();
        }
    }
}
