// Code generated from app/lib/l10n/app_zh.arb; DO NOT EDIT.

package l10n

import "fmt"

var _ = fmt.Sprint

func AboutCheckFailed(e any) string {
	return "无法检查更新：" + fmt.Sprint(e) + "。请检查网络后重试。"
}

const AboutCheckHint = "从 GitHub Releases 检查是否有新版本"

const AboutCheckUpdates = "检查更新"

const AboutChecking = "正在检查…"

func AboutDownload(version any) string {
	return "下载 " + fmt.Sprint(version)
}

const AboutFeedback = "反馈问题"

func AboutFeedbackBody(version any, os any) string {
	return "请描述遇到的问题和复现步骤：\n\n\n---\n版本：" + fmt.Sprint(version) + "\n系统：" + fmt.Sprint(os)
}

const AboutFeedbackHint = "在 GitHub 提交问题，自动附上版本和系统"

const AboutLicenses = "开源许可"

const AboutLicensesHint = "Coursive 用到的开源组件"

const AboutLinks = "链接"

func AboutNewVersion(version any, when any) string {
	return "有新版本 " + fmt.Sprint(version) + "（" + fmt.Sprint(when) + "发布）"
}

const AboutPreview = "预览版"

const AboutReleaseNotes = "查看更新说明"

const AboutRepo = "代码仓库"

const AboutReward = "赞赏"

const AboutRewardBody = "觉得 Coursive 有用，可以用微信扫码赞赏。"

const AboutRewardImage = "微信赞赏码"

const AboutRewardPhoneHint = "在本机上可以先截屏，再在微信「扫一扫」里从相册识别。"

const AboutRewardTitle = "请作者喝杯咖啡"

const AboutTagline = "课堂录像的回放、转写与笔记"

const AboutUpToDate = "已是最新版本"

const AboutUpdates = "更新"

func AboutVersion(value any) string {
	return "版本 " + fmt.Sprint(value)
}

func ActivityActiveDays(activeDays any) string {
	return fmt.Sprint(activeDays) + " 天"
}

func ActivityAsks(asks any) string {
	return "提问 " + fmt.Sprint(asks) + " 次"
}

const ActivityByCourse = "按课程"

const ActivityByCourseEmpty = "还没有按课程记录的学习时间"

const ActivityByKind = "按类型"

const ActivityCurrentStreak = "当前连续"

const ActivityDayEmpty = "这天没有学习记录"

func ActivityDayLabel(value any, value2 any) string {
	return fmt.Sprint(value) + "，" + fmt.Sprint(value2)
}

func ActivityDaySummary(quizzes any, asks any, notes any) string {
	return "完成练习 " + fmt.Sprint(quizzes) + " 组 · 向助手提问 " + fmt.Sprint(asks) + " 次 · 新建笔记 " + fmt.Sprint(notes) + " 篇"
}

const ActivityDaysCount = "学习天数"

const ActivityDuration = "学习时长"

const ActivityEmpty = "开始看课或做题后，这里会记录你的学习足迹"

const ActivityHasRecord = "有学习记录"

func ActivityLongestDays(longest any) string {
	return fmt.Sprint(longest) + " 天"
}

const ActivityLongestStreak = "最长连续"

const ActivityNextYear = "下一年"

const ActivityNoStudy = "没有学习"

const ActivityNone = "没有学习记录"

func ActivityNotes(value any) string {
	return "笔记 " + fmt.Sprint(value) + " 篇"
}

func ActivityNotesCreated(notesCreated any) string {
	return "新建笔记 " + fmt.Sprint(notesCreated) + " 篇"
}

func ActivityNotesEdited(notesEdited any) string {
	return "编辑笔记 " + fmt.Sprint(notesEdited) + " 篇"
}

const ActivityPrevYear = "上一年"

func ActivityQuizzes(quizzes any) string {
	return "练习 " + fmt.Sprint(quizzes) + " 组"
}

func ActivityQuizzesDone(quizzes any) string {
	return "完成练习 " + fmt.Sprint(quizzes) + " 组"
}

const ActivityReading = "阅读"

const ActivityStatsTitle = "学习统计"

const ActivityStreak = "连续学习"

func ActivityStudyTime(value any) string {
	return "学习 " + fmt.Sprint(value)
}

const ActivityThisWeek = "本周"

func ActivityTranscripts(transcripts any) string {
	return "完成转写 " + fmt.Sprint(transcripts) + " 节"
}

const ActivityTranscriptsHeader = "完成转写"

const ActivityUntimed = "没有计时的学习"

const ActivityWatching = "看课"

func ActivityYear(year any) string {
	return fmt.Sprint(year) + " 年"
}

const AddProviderBuiltInKeyOnly = "Pi SDK 内置，只需密钥"

const AddProviderCatalog = "models.dev 目录"

func AddProviderCatalogEntry(value any, value2 any) string {
	return fmt.Sprint(value) + " 个模型 · " + fmt.Sprint(value2)
}

func AddProviderCatalogError(catalogError any) string {
	return "models.dev 目录暂时不可用：" + fmt.Sprint(catalogError)
}

const AddProviderCatalogLoading = "正在加载 models.dev 目录…"

const AddProviderCompatible = "兼容接口"

const AddProviderCompatibleHint = "任何兼容 OpenAI / Anthropic / Gemini 接口的地址，包括中转和自建服务"

const AddProviderLoadingCatalog = "没有匹配的预设，正在加载 models.dev 目录…"

func AddProviderMatches(length any) string {
	return fmt.Sprint(length) + " 个匹配"
}

func AddProviderNoMatch(q any) string {
	return "没有找到“" + fmt.Sprint(q) + "”。可以用下方「自定义服务」手动填写。"
}

const AddProviderNotFound = "找不到？"

const AddProviderSearchHint = "搜索服务商，例如 硅基、qwen、groq"

func AddProviderSearchesMore(moreCount any) string {
	return "搜索时还会在 models.dev 收录的另外 " + fmt.Sprint(moreCount) + " 家服务商中查找。"
}

const ApiOpenaiCompatible = "OpenAI Chat Completions 兼容"

const AppDaemonFailed = "后台服务无法启动"

const AppRestart = "重新启动"

const AppTitle = "Coursive"

const ApprovalAsk = "询问"

const ApprovalAskDetail = "改动先生成卡片，由你应用"

const ApprovalAuto = "自动"

const ApprovalAutoDetail = "直接执行，可以撤销"

const ApprovalDeleteNote = "删除文件在两种模式下都会先问你"

const ApprovalFailed = "无法切换操作模式。确认助手已启动后再试"

const ApprovalSwitchHint = "Shift+Tab 切换"

const ApprovalTooltip = "操作模式"

const AsrBalanced = "均衡"

func AsrBusy(value any) string {
	return "识别中 " + fmt.Sprint(value) + " 段"
}

const AsrChangeLocation = "更改位置"

const AsrChangeOnly = "只更改位置"

const AsrChangeOnlyBody = "只更改位置：旧文件留在原处，之后下载到新位置；新位置里已有的模型会直接可用。"

func AsrChannelLimits(value any) string {
	return "渠道上限：" + fmt.Sprint(value) + "；调高不一定更快，容易触发限流"
}

func AsrChannelSlots(value any, value2 any) string {
	return fmt.Sprint(value) + " " + fmt.Sprint(value2) + " 段"
}

const AsrConcurrency = "同时识别的段数"

func AsrCooling(value any) string {
	return "冷却至 " + fmt.Sprint(value)
}

const AsrCustomLocation = "自定义位置"

const AsrDefaultLocation = "默认位置"

const AsrDeleteVideo = "转写完成后删除视频"

const AsrDeleteVideoHint = "只保留文字稿、PPT 和课件，视频文件在转写成功后立即删除，节省磁盘空间。在线播放不受影响。"

const AsrDistribution = "分配方式"

func AsrDownloadingModel(value any) string {
	return "正在下载 " + fmt.Sprint(value)
}

const AsrDownloadingModelBody = "下载完成后会自动启用。可以离开这个页面，进度会在后台继续。"

const AsrEngineProxy = "加速前缀（可选）"

const AsrEngineSource = "识别引擎（GitHub）"

const AsrEngineSourceBody = "第一次下载时会从 GitHub Releases 取 sherpa-onnx 识别引擎（约 25 MB）。GitHub 很慢时，可以填一个 GitHub 下载加速服务的前缀，实际地址为「前缀 + 原始地址」，同样会校验 SHA-256。"

func AsrEventLine(value any, value2 any, value3 any, value4 any, value5 any) string {
	return fmt.Sprint(value) + "  " + fmt.Sprint(value2) + " · 第 " + fmt.Sprint(value3) + " 段 · " + fmt.Sprint(value4) + "音频 · 用时 " + fmt.Sprint(value5) + " 秒"
}

func AsrFailedSegments(value any) string {
	return "失败 " + fmt.Sprint(value) + " 段"
}

func AsrFreeSpace(value any) string {
	return "磁盘剩余 " + fmt.Sprint(value)
}

const AsrHowToChoose = "怎么选"

const AsrHowToChooseBody = "三项都是越短越好。点一行可以在下面看到详情。没有特别需求就选带 ★ 的推荐模型。"

const AsrIntro = "长音频会切成 20 分钟的段，分给下面的渠道识别；某个渠道被限流时自动冷却，由其他渠道接手。已识别的段会保留，重试时只补缺失的部分。"

const AsrLocalEnabled = "本地识别已启用"

const AsrLocalHeroBody = "下载一个语音识别模型到本机，转写不再受免费云端渠道的限流和额度影响，音频也不会离开本机。"

const AsrLocalHeroTitle = "在本机识别"

const AsrLocalHint = "下载一个识别模型到本机，看对比图选适合你电脑的那个"

const AsrLocalTitle = "本地识别模型"

func AsrLocalUnlimited(value any) string {
	return fmt.Sprint(value) + " · 不受额度限制"
}

const AsrLocationAfterDownload = "下载完成后才能更改位置"

const AsrLowSpace = "这块磁盘剩余空间不多，模型可能下载不完。可以换到空间更大的磁盘。"

const AsrModelAccurate = "最准"

const AsrModelAutoPunctuation = "自动补标点"

func AsrModelCer(value any) string {
	return "错字率 " + fmt.Sprint(value) + "%"
}

func AsrModelCerSource(value any) string {
	return "错字率来源：" + fmt.Sprint(value)
}

func AsrModelDeleteBody(value any) string {
	return "会释放 " + fmt.Sprint(value) + " 空间。已经转写好的文字稿不受影响，之后可以重新下载。"
}

func AsrModelDeleteTitle(value any) string {
	return "删除 " + fmt.Sprint(value) + "？"
}

func AsrModelDownload(value any, value2 any) string {
	return "下载 " + fmt.Sprint(value) + fmt.Sprint(value2)
}

func AsrModelDownloadFailed(value any) string {
	return "下载失败：" + fmt.Sprint(value)
}

const AsrModelFastest = "最快"

const AsrModelIncludesEngine = "（含引擎等）"

const AsrModelLocation = "模型存储位置"

const AsrModelLocationChanged = "之后下载的模型会保存到新位置"

func AsrModelMemory(value any) string {
	return "内存约 " + fmt.Sprint(value)
}

const AsrModelOriginal = "原始模型"

const AsrModelPunctuated = "自带标点"

const AsrModelRecommended = "推荐"

const AsrModelResume = "继续下载"

func AsrModelSpeed(value any) string {
	return "90 分钟课约 " + fmt.Sprint(value) + " 分钟"
}

const AsrModelUse = "使用这个模型"

const AsrMove = "移动"

func AsrMoveBody(value any) string {
	return "移动：" + fmt.Sprint(value) + " 的引擎和模型会搬到新位置，旧位置随后清空。跨磁盘时需要复制一会儿，期间不能转写。"
}

const AsrMoveTitle = "把已下载的模型一起移过去？"

const AsrMoving = "正在移动模型"

const AsrMovingBody = "移动完成前本地识别暂停，云端渠道照常转写。可以离开这个页面。"

func AsrOkSegments(value any) string {
	return "成功 " + fmt.Sprint(value) + " 段"
}

const AsrPerChannelLimits = "每个渠道有各自的并发上限"

func AsrPrefer(value any) string {
	return "优先" + fmt.Sprint(value)
}

const AsrQueued = "已加入转写队列"

const AsrQueuedShowLater = "已加入转写队列，完成后自动显示"

const AsrQuotaFull = "额度已满"

func AsrQuotaUsage(value any, value2 any) string {
	return "12 小时内 " + fmt.Sprint(value) + " / " + fmt.Sprint(value2)
}

const AsrReady = "就绪"

const AsrRecent = "最近识别记录"

func AsrRecentSummary(length any, fails any) string {
	return "最近 " + fmt.Sprint(length) + " 次 · 失败 " + fmt.Sprint(fails) + " 次"
}

const AsrResetLocation = "恢复默认"

func AsrSegments(value any) string {
	return fmt.Sprint(value) + " 段"
}

const AsrSource = "下载来源"

const AsrSourceCustom = "适合学校、实验室或自建的 Hugging Face 兼容镜像。填写地址模板，{repo} 会换成模型仓库名，{file} 换成文件路径，例如 https://hf-mirror.com/{repo}/resolve/main/{file}。"

const AsrSourceHfMirror = "无需任何配置。hf-mirror.com 是社区维护的 Hugging Face 镜像，国内大多数网络可以直连，目录里的全部模型都能从这里下载。"

const AsrSourceHuggingface = "官方源，文件最全，但国内通常需要代理。Coursive 的后台服务会使用系统的 HTTPS_PROXY 环境变量。"

const AsrSourceModelscope = "无需账号或 Token，公开模型直接下载。魔搭上目前只有 Qwen3-ASR 0.6B 和语音检测模型有与原版逐字节相同的副本（社区用户上传），选它们时走魔搭；其余文件自动改用 HF-Mirror，下载时会提示。"

const AsrSourceTemplate = "地址模板"

const AsrSourceVerify = "所有文件都会按目录里记录的 SHA-256 校验，来源被篡改或版本不一致时会直接报错，不会装上。中断后再次下载会从断点继续。"

const AsrStart = "开始转写"

const AsrStorage = "存储位置"

func AsrThreadCount(value any) string {
	return fmt.Sprint(value) + " 线程"
}

const AsrThreads = "识别线程"

func AsrThreadsHint(cpus any) string {
	return "线程越多越快，但转写时设备会更忙、更费电。本机有 " + fmt.Sprint(cpus) + " 个逻辑核心，默认用一半（最多 4 个）。"
}

const AsrTryLocal = "免费渠道限流太厉害？改用本地模型"

func AsrUnsupportedBody(value any) string {
	return "本地识别引擎没有提供 " + fmt.Sprint(value) + " 的预编译版本，转写会继续使用免费云端渠道。"
}

const AsrUnsupportedTitle = "本机暂不支持"

func AsrUsedSpace(value any) string {
	return "已用 " + fmt.Sprint(value)
}

func AsrUsingModel(value any) string {
	return "正在使用 " + fmt.Sprint(value)
}

const AsrUsingModelBody = "转写会交给本机模型，不受限流和额度影响；开着的云端渠道也会并行分担一部分。想让音频完全不离开本机，在「设置 → 转写」里关掉云端渠道。"

const AssetAsr = "语音转写"

const AssetAudio = "音频"

const AssetCourseware = "课件"

const AssetParse = "课件解析"

const AssetSlides = "PPT 截图"

const AssetTranscript = "转写"

const AssetVideo = "视频"

func AssignmentDue(value any, label any) string {
	return "截止 " + fmt.Sprint(value) + " · " + fmt.Sprint(label)
}

func AssignmentDueShort(value any) string {
	return "截止 " + fmt.Sprint(value)
}

const AssignmentGraded = "已批改"

func AssignmentGradedScore(value any) string {
	return "已批改 · " + fmt.Sprint(value) + " 分"
}

const AssignmentMissing = "已截止 · 未提交"

const AssignmentNoDue = "没有截止时间"

func AssignmentOpenCount(length any) string {
	return fmt.Sprint(length) + " 项待交"
}

func AssignmentPoints(pts any) string {
	return fmt.Sprint(pts) + " 分"
}

func AssignmentTimeLeft(value any) string {
	return "还剩 " + fmt.Sprint(value)
}

const AssistantNotStarted = "助手没有启动"

const AssistantSetupModel = "先配置一个模型"

const AssistantSetupModelAction = "配置模型"

const AssistantSetupModelBody = "助手通过 Pi SDK 调用你自己的模型服务。填入 API Key，或导入 Pi CLI 的配置。"

const AssistantStarting = "正在启动助手"

const AvatarShapeCircle = "圆形"

const AvatarShapeCookie = "曲奇"

const AvatarShapePetal = "花瓣"

const AvatarShapeSquircle = "圆角方"

func BackgroundJobs(active any) string {
	return "正在处理 " + fmt.Sprint(active) + " 个下载或转写任务"
}

const BackgroundSyncing = "正在同步数据"

const BackupExport = "导出备份"

func BackupExportFailed(err any) string {
	return "导出失败：" + fmt.Sprint(err)
}

func BackupExportSize(value any) string {
	return "导出备份 · 约 " + fmt.Sprint(value)
}

func BackupFileName(year any, value any, value2 any) string {
	return "Coursive-备份-" + fmt.Sprint(year) + fmt.Sprint(value) + fmt.Sprint(value2) + ".zip"
}

func BackupFromDevice(device any) string {
	return "自 " + fmt.Sprint(device)
}

const BackupIncludeMedia = "包含视频与录音"

const BackupIntro = "把课程库和助手对话打包成一个 zip，换电脑或重装时恢复。登录状态和密钥不在备份里。"

func BackupMediaSize(value any, value2 any) string {
	return fmt.Sprint(value) + " 个文件，另需 " + fmt.Sprint(value2)
}

const BackupNoMedia = "课程库里还没有视频"

const BackupNotSaved = "没有保存备份"

const BackupNothingToRestore = "没有需要恢复的文件，本机已是最新"

func BackupPreviewMeta(value any, value2 any, value3 any, value4 any, value5 any, value6 any) string {
	return fmt.Sprint(value) + " " + fmt.Sprint(value2) + ":" + fmt.Sprint(value3) + " 导出" + fmt.Sprint(value4) + " · " + fmt.Sprint(value5) + " 个文件 · " + fmt.Sprint(value6)
}

const BackupReading = "正在读取备份"

const BackupRestore = "恢复备份"

func BackupRestoreFailed(e any) string {
	return "恢复失败：" + fmt.Sprint(e)
}

const BackupRestoreFrom = "从备份恢复"

func BackupRestored(value any, value2 any) string {
	return "已恢复 " + fmt.Sprint(value) + " 个文件（" + fmt.Sprint(value2) + "）"
}

const BackupRestoring = "正在恢复"

func BackupSaved(value any) string {
	return "备份已保存（" + fmt.Sprint(value) + "）"
}

const BackupTitle = "备份与恢复"

const BackupTypeLabel = "Coursive 备份"

const BackupViewReplaced = "查看被替换的文件"

const BenchCer = "错字率"

const BenchCerSource = "WenetSpeech 会议测试集（最接近课堂录音）的字错误率，取自各模型的论文；来源不同，相差 1% 以内可视为持平"

const BenchLowerIsBetter = "越短越准"

const BenchMemory = "内存"

const BenchMemoryHint = "识别时的峰值"

const BenchMemorySource = "同一次实测的进程峰值内存"

func BenchMetricSource(label any, source any) string {
	return fmt.Sprint(label) + "：" + fmt.Sprint(source)
}

func BenchMinutes(value any) string {
	return fmt.Sprint(value) + " 分钟"
}

const BenchReferenceMachine = "参考机器"

func BenchRowLabel(name any, summary any) string {
	return fmt.Sprint(name) + "，" + fmt.Sprint(summary)
}

func BenchSortBy(label any) string {
	return "按" + fmt.Sprint(label)
}

const BenchTime = "转写用时"

const BenchTimeHint = "一节 90 分钟的课"

func BenchTimeSource(value any, value2 any) string {
	return "在 " + fmt.Sprint(value) + " 上实测（" + fmt.Sprint(value2) + "，含加载模型），你的设备会有不同"
}

const BenchUntested = "未测"

const BookmarksHint = "按 B 键或点播放器上的书签按钮标记重点"

const BotAccessoryAntenna = "天线"

const BotAccessoryBunnyEars = "兔耳"

const BotAccessoryCatEars = "猫耳"

const BotAccessoryDrop = "水滴"

const BotAccessoryFeelers = "触角"

const BotAccessoryGhost = "小幽灵"

const BotAccessoryHeadphones = "耳机"

const BotAccessorySprout = "豆芽"

const BotDefaultName = "小 C"

const BotIntroBody = "我会记住你告诉我的事，也能替你在应用里动手：下载、调设置、整理课程、安排定时任务，做完都可以撤销。想先过目，就在输入框旁切到「询问」；删除文件总会先问你。"

func BotIntroTitle(botName any) string {
	return "我是 " + fmt.Sprint(botName)
}

const BotMoodConfused = "出了点问题"

const BotMoodDreaming = "在整理记忆"

const BotMoodHappy = "完成了"

const BotMoodIdle = "在这儿"

func BotMoodLabel(botName any, value any) string {
	return fmt.Sprint(botName) + "，" + fmt.Sprint(value)
}

const BotMoodListening = "在听"

const BotMoodNotify = "有卡片等你确认"

const BotMoodTalking = "在说"

const BotMoodThinking = "在想"

const BotMoodWorking = "在做事"

const BotName = "名字"

const BotNoCourse = "没有指明要打开哪门课"

const BotNoDownload = "没有指明要下载哪一节课"

const BotNoLecture = "没有指明要打开哪一节课"

const BotNoNote = "没有指明要打开哪篇笔记"

func BotOpened(value any) string {
	return "已打开 " + fmt.Sprint(value)
}

const BotPaused = "已暂停播放"

const BotPersonaHint = "比如：说话轻松一点，先给结论；复习时多出几道题考我。"

func BotRateSet(value any) string {
	return "已调到 " + fmt.Sprint(value) + "×"
}

const BotRestart = "重新开始"

func BotRestartBody(botName any) string {
	return fmt.Sprint(botName) + " 的对话会从头开始，记忆保留。"
}

const BotRestartChat = "重新开始对话"

const BotRestartTitle = "重新开始对话？"

const BotResumed = "已继续播放"

const BotSectionLook = "形象"

const BotSectionMemory = "记忆"

const BotSectionPersona = "相处方式"

const BotSeeked = "已跳转"

const BotUnsupported = "这个版本还不支持该操作"

const CatalogAllCategories = "全部类别"

const CatalogAllTypes = "全部文件类型"

const CatalogCategory = "用途类别"

const CatalogClassificationHint = "依据文件名与目录自动分类；原文件保留原位。展开类别查看资料。"

const CatalogCourseware = "课件"

const CatalogEmpty = "没有符合筛选条件的资料。"

const CatalogFileType = "文件类型"

func CatalogFulltextTitle(count int) string {
	return "正文检索（按课程）· " + fmt.Sprint(count) + " 条"
}

const CatalogHomework = "作业"

const CatalogLab = "Lab / 实验"

func CatalogLoadFailed(error string) string {
	return "无法读取资料列表：" + error
}

const CatalogOther = "其他"

const CatalogReference = "参考资料"

const CatalogSearchHint = "搜索文件名；同时检索文字稿与笔记正文"

const CatalogSlides = "课堂截图"

func CatalogTitle(count int) string {
	return "资料 · " + fmt.Sprint(count) + " 项"
}

const CatalogVideo = "录像"

const ChartTotal = "合计"

const ChatAllSessions = "全部会话"

const ChatCodeCopied = "已复制代码"

const ChatContextPlaying = "正在播放 · 助手可见当前进度"

func ChatContextScreen(value any) string {
	return "当前：" + fmt.Sprint(value)
}

const ChatCurrentCourse = "当前课程"

const ChatInputHint = "问点什么…（Shift+Enter 换行）"

const ChatMoreTools = "更多功能"

const ChatNew = "新对话"

const ChatNewSession = "新会话"

const ChatNoSessions = "还没有会话"

func ChatOpenFailed(e any) string {
	return "无法打开对话：" + fmt.Sprint(e)
}

const ChatOtherSessions = "其他对话"

func ChatOutlineChapters(length any) string {
	return "大纲 · " + fmt.Sprint(length) + " 个章节"
}

const ChatSavedToLibrary = "已保存到课程库 · 点击阅读"

const ChatSend = "发送 (Enter)"

func ChatSessionMeta(value any, messages any) string {
	return fmt.Sprint(value) + " · " + fmt.Sprint(messages) + " 条"
}

const ChatSessions = "会话"

func ChatSlideRef(value any) string {
	return "PPT 第 " + fmt.Sprint(value) + " 张"
}

func ChatSwitchCourse(value any) string {
	return "改为「" + fmt.Sprint(value) + "」的课程助手"
}

const ChatThinking = "正在思考…"

const ChatThinkingNow = "思考中"

const ChatThinkingProcess = "思考过程"

const ChatVisionTranscribed = "由识图模型转写给助手"

const ChatVisionUnavailable = "当前模型不能看图，可以在「设置 → 助手 → 识图模型」里设置"

const ChatWelcomeBody = "我能读取课表、文字稿和笔记，帮你整理内容、定位知识点。下载由你点按钮决定。"

const ChatWelcomeTitle = "有什么可以帮你？"

const ColorAmber = "琥珀"

const ColorAzure = "湛蓝"

const ColorBegonia = "海棠"

const ColorCinnabar = "朱砂"

const ColorGraphite = "石墨"

const ColorIndigo = "黛青"

const ColorPine = "松绿"

const ColorWisteria = "藤紫"

func CommonActionFailed(e any) string {
	return "操作失败：" + fmt.Sprint(e)
}

const CommonActivity = "学习足迹"

const CommonAdd = "添加"

const CommonAdvanced = "高级"

const CommonAll = "全部"

const CommonAllEnabled = "全部启用"

const CommonAllow = "允许"

const CommonApply = "应用"

const CommonAssignments = "作业"

const CommonAssistant = "助手"

const CommonAuto = "自动"

const CommonAutoSuffix = " · 自动"

const CommonAutoSync = "自动同步"

const CommonBack = "返回"

const CommonBookmarks = "书签"

const CommonBuiltIn = "内置"

const CommonCancel = "取消"

const CommonCancelled = "已取消"

const CommonCannotOpenFolder = "无法打开文件夹"

const CommonChange = "更换"

const CommonChecking = "正在检查"

const CommonChoose = "选择"

const CommonChooseFiles = "选择文件"

const CommonClean = "清理"

const CommonClear = "清除"

const CommonClose = "关闭"

const CommonCollapse = "收起"

const CommonColor = "颜色"

const CommonConnect = "连接"

const CommonConnected = "已连接"

const CommonConnecting = "正在连接"

const CommonConnectionOk = "连接正常"

const CommonContents = "目录"

const CommonCopied = "已复制"

const CommonCopy = "复制"

const CommonCopyAll = "复制全部"

const CommonCopyPath = "复制路径"

const CommonCourses = "课程"

const CommonCustom = "自定义"

const CommonDaily = "每天"

const CommonDate = "日期"

const CommonDayAfterTomorrow = "后天"

func CommonDays(current any) string {
	return fmt.Sprint(current) + " 天"
}

const CommonDelete = "删除"

func CommonDeleteFailed(e any) string {
	return "删除失败：" + fmt.Sprint(e)
}

const CommonDeleted = "已删除"

const CommonDeselectAll = "取消全选"

const CommonDisable = "停用"

const CommonDisabled = "已停用"

const CommonDisconnect = "断开"

const CommonDone = "已完成"

const CommonDownload = "下载"

func CommonDownloadFailed(e any) string {
	return "下载失败：" + fmt.Sprint(e)
}

func CommonDownloadSize(value any) string {
	return "下载 " + fmt.Sprint(value)
}

const CommonDownloaded = "已下载"

const CommonEdit = "编辑"

const CommonEmpty = "清空"

const CommonEnable = "启用"

const CommonEnabled = "已启用"

const CommonExport = "导出"

func CommonExportFailed(e any) string {
	return "导出失败：" + fmt.Sprint(e)
}

const CommonExportNow = "立即导出"

const CommonExporting = "正在导出"

const CommonFailed = "失败"

func CommonFilePickerFailed(message any) string {
	return "无法打开文件选择器：" + fmt.Sprint(message)
}

func CommonFilesCount(length any) string {
	return fmt.Sprint(length) + " 个文件"
}

const CommonFind = "查找"

const CommonFinished = "完成"

const CommonFolder = "文件夹"

const CommonFollowTheme = "跟随主题"

const CommonGetStarted = "开始"

const CommonHide = "隐藏"

const CommonHidePassword = "隐藏密码"

const CommonIgnore = "忽略"

const CommonImage = "图片"

const CommonInUse = "使用中"

const CommonInstall = "安装"

const CommonInstalled = "已安装"

func CommonLastSynced(value any) string {
	return "上次同步 " + fmt.Sprint(value)
}

func CommonLastUsed(value any) string {
	return "上次 " + fmt.Sprint(value)
}

const CommonLecture = "课次"

const CommonLibrary = "课程库"

func CommonLicense(value any) string {
	return "许可：" + fmt.Sprint(value)
}

const CommonLoading = "加载中"

const CommonLocal = "本地"

const CommonLocation = "位置"

const CommonMirror = "下载源"

const CommonModel = "模型"

const CommonModify = "修改"

const CommonMore = "更多"

const CommonName = "名称"

const CommonNeedsAttention = "需要处理"

const CommonNew = "新建"

const CommonNewBadge = "新"

const CommonNext = "下一步"

const CommonNoData = "没有数据"

const CommonNoRecords = "没有记录"

const CommonNone = "无"

const CommonNotConfigured = "未配置"

const CommonNotConnected = "未连接"

const CommonNotInstalled = "未安装"

const CommonNotSet = "未设置"

const CommonNotes = "笔记"

const CommonNotifications = "通知"

const CommonOpen = "打开"

const CommonOpenFolder = "打开文件夹"

const CommonOpenInFileManager = "在文件管理器中打开"

const CommonOther = "其他"

const CommonOutline = "大纲"

func CommonPages(value any) string {
	return fmt.Sprint(value) + " 页"
}

const CommonPassword = "密码"

const CommonPaste = "粘贴"

const CommonPathCopied = "已复制路径"

const CommonPause = "暂停"

const CommonPaused = "已暂停"

const CommonPlay = "播放"

const CommonPlugins = "插件"

const CommonPractice = "练习"

const CommonPreview = "预览"

const CommonQueued = "排队中"

const CommonRefresh = "刷新"

const CommonRegenerate = "重新生成"

const CommonRemove = "移除"

const CommonRename = "重命名"

const CommonRestore = "恢复"

const CommonResume = "继续"

const CommonRetry = "重试"

const CommonRunning = "运行中"

const CommonSave = "保存"

func CommonSaveFailed(e any) string {
	return "保存失败：" + fmt.Sprint(e)
}

const CommonSaved = "已保存"

const CommonSaving = "保存中…"

const CommonSavingShort = "正在保存"

const CommonSchedule = "日程"

const CommonScheduledTasks = "定时任务"

func CommonSecondsValue(seekStepSec any) string {
	return fmt.Sprint(seekStepSec) + " 秒"
}

const CommonSelectAll = "全选"

const CommonSettings = "设置"

const CommonShow = "显示"

const CommonShowInFolder = "在文件夹中显示"

const CommonShowPassword = "显示密码"

const CommonStartSync = "开始同步"

const CommonStarting = "正在启动"

const CommonStop = "停止"

const CommonSync = "同步"

const CommonSyncNow = "立即同步"

func CommonSyncProgress(done any, total any) string {
	return "正在同步 " + fmt.Sprint(done) + " / " + fmt.Sprint(total)
}

const CommonSyncing = "正在同步…"

const CommonTasks = "任务"

func CommonTasksCount(length any) string {
	return "任务 " + fmt.Sprint(length)
}

const CommonTest = "测试"

const CommonTime = "时间"

func CommonTimeLeft(value any) string {
	return "还需 " + fmt.Sprint(value)
}

const CommonTitle = "标题"

const CommonToday = "今天"

const CommonTomorrow = "明天"

const CommonTranscript = "文字稿"

const CommonUncategorized = "未归类"

const CommonUndo = "撤销"

const CommonUndone = "已撤销"

const CommonUninstall = "卸载"

const CommonUnlimited = "不限"

const CommonUntitled = "无标题"

func CommonUpdateFailed(e any) string {
	return "更新失败：" + fmt.Sprint(e)
}

const CommonUpload = "上传"

const CommonUsage = "用量"

const CommonUse = "使用"

const CommonView = "查看"

const CommonWeekly = "每周"

const CommonYesterday = "昨天"

func ContextCacheHit(value any) string {
	return " · 缓存命中 " + fmt.Sprint(value)
}

const ContextCompactHint = "接近上限时 Pi 会自动压缩较早的对话。"

const ContextCompactNow = "立即压缩上下文"

func ContextCost(cost any) string {
	return "估算费用 $" + fmt.Sprint(cost)
}

func ContextTokens(value any, value2 any, value3 any) string {
	return "本对话累计 输入 " + fmt.Sprint(value) + " · 输出 " + fmt.Sprint(value2) + fmt.Sprint(value3)
}

func ContextUsage(value any, value2 any, value3 any) string {
	return "上下文 " + fmt.Sprint(value) + " / " + fmt.Sprint(value2) + "（" + fmt.Sprint(value3) + "%）"
}

func ContextUsageTooltip(value any) string {
	return "上下文用量 " + fmt.Sprint(value) + "%"
}

const CourseAssistant = "课程助手"

func CourseAssistantIntro(botName any) string {
	return "这个会话只围绕这门课：讲解、找知识点、整理笔记、出题。关于你的偏好来自 " + fmt.Sprint(botName) + " 的记忆。"
}

func CourseAssistantTitle(value any) string {
	return fmt.Sprint(value) + "助手"
}

const CourseBatchDownload = "批量下载"

const CourseContinue = "继续学习"

func CourseDownloadSelected(length any) string {
	return "下载 " + fmt.Sprint(length) + " 节"
}

func CourseDownloadedKind(label any) string {
	return "已下载" + fmt.Sprint(label)
}

func CourseLectureCount(lectureTotal any) string {
	return fmt.Sprint(lectureTotal) + " 节"
}

func CourseLectureTotal(length any) string {
	return fmt.Sprint(length) + " 节课"
}

const CourseLoadingSchedule = "正在读取课表"

const CourseNewestFirst = "最新在前"

const CourseOldestFirst = "第一节在前"

func CourseProgress(value any) string {
	return "学习进度 " + fmt.Sprint(value) + "%"
}

const CourseRefreshSchedule = "刷新课表"

func CourseSelectedCount(length any) string {
	return "已选 " + fmt.Sprint(length) + " 节"
}

const CourseStart = "开始学习"

func CourseStatLectures(lectures any) string {
	return fmt.Sprint(lectures) + " 节"
}

func CourseStatLocal(local any) string {
	return "本地 " + fmt.Sprint(local) + " 节"
}

func CourseTitleQuoted(title any) string {
	return "《" + fmt.Sprint(title) + "》"
}

const CourseUnscheduled = "未排期"

const CoursesAllHidden = "课程都已隐藏"

const CoursesAllTerms = "全部学期"

const CoursesClearSearch = "清除搜索"

const CoursesColor = "课程颜色"

const CoursesDisplayName = "显示名称"

const CoursesDisplayNameHelper = "留空则使用原名，不影响本地文件夹"

const CoursesEmptyBody = "新建一门课程，从本机导入课堂录像，就能开始看课、记笔记和复习。"

const CoursesEmptySources = "安装来源插件"

const CoursesEmptyTerm = "这个学期没有课程"

const CoursesEmptyTitle = "还没有课程"

func CoursesHiddenCount(hiddenCount any) string {
	return "已隐藏 " + fmt.Sprint(hiddenCount)
}

func CoursesNoMatch(text any) string {
	return "没有匹配“" + fmt.Sprint(text) + "”的课程"
}

func CoursesOriginalName(rawTitle any) string {
	return "原名：" + fmt.Sprint(rawTitle)
}

const CoursesPin = "置顶"

const CoursesRenameTitle = "重命名课程"

const CoursesSearchHint = "搜索课程或老师"

const CoursesShowHidden = "显示已隐藏的课程"

const CoursesSync = "从来源同步课程"

const CoursesSyncing = "正在加载课程"

const CoursesUnhide = "取消隐藏"

const CoursesUnpin = "取消置顶"

const CoursesViewAllTerms = "查看全部学期"

func CoursewareDeleteBody(name any) string {
	return "「" + fmt.Sprint(name) + "」和它的解析结果会从课程库中删除。"
}

const CoursewareDeleteTitle = "删除课件？"

const CoursewareGone = "课件已不存在"

const CoursewareNotParsed = "这个课件还没有解析"

const CoursewareNotParsedState = "未解析"

const CoursewareOpenOriginal = "打开原文件"

func CoursewarePages(pages any) string {
	return " · " + fmt.Sprint(pages) + " 页"
}

func CoursewarePagesSuffix(pages any) string {
	return " " + fmt.Sprint(pages) + " 页"
}

const CoursewareParse = "解析"

const CoursewareParseFailed = "解析失败"

const CoursewareParseQueued = "等待解析"

const CoursewareParseQueuedToast = "已加入解析队列"

const CoursewareParseTooltip = "用 MinerU 解析"

func CoursewareParsed(value any) string {
	return "已解析" + fmt.Sprint(value)
}

func CoursewareParsedBy(value any) string {
	return "MinerU 解析" + fmt.Sprint(value)
}

const CoursewareParsing = "正在解析…"

func CoursewareReadFailed(statusCode any) string {
	return "读取失败：HTTP " + fmt.Sprint(statusCode)
}

const CoursewareReadParsed = "阅读解析结果"

const CoursewareReparse = "重新解析"

const CoursewareUploadBody = "课程来源通常不提供课件。把老师发的 PDF、PPT、Word 传上来，MinerU 会把它解析成文字，可以全文阅读，助手也能引用。"

func CoursewareUploadFailed(value any, e any) string {
	return fmt.Sprint(value) + " 上传失败：" + fmt.Sprint(e)
}

const CoursewareUploadTitle = "上传这节课的课件"

func CoursewareUploaded(ok any) string {
	return "已上传 " + fmt.Sprint(ok) + " 个文件"
}

func CoursewareUploadedParsing(ok any) string {
	return "已上传 " + fmt.Sprint(ok) + " 个文件，正在用 MinerU 解析"
}

func DaemonCannotReadAppDir(e any) string {
	return "无法读取应用目录：" + fmt.Sprint(e)
}

func DaemonCannotStart(e any) string {
	return "无法启动后台服务：" + fmt.Sprint(e)
}

func DaemonKeepsExiting(code any) string {
	return "后台服务反复退出（退出码 " + fmt.Sprint(code) + "），可以重新启动再试。"
}

const DaemonNotFound = "找不到后台服务 coursive-daemon。开发环境请先构建：cd daemon && go build -o bin/coursive-daemon ./cmd/coursive-daemon"

const DataSyncTitle = "多端同步"

func DateMonth(month any) string {
	return fmt.Sprint(month) + "月"
}

func DateMonthDay(month any, day any) string {
	return fmt.Sprint(month) + "月" + fmt.Sprint(day) + "日"
}

func DateMonthDayTime(month any, day any, value any, value2 any) string {
	return fmt.Sprint(month) + "月" + fmt.Sprint(day) + "日 " + fmt.Sprint(value) + ":" + fmt.Sprint(value2)
}

func DateMonthDayWeek(month any, day any, value any) string {
	return fmt.Sprint(month) + "月" + fmt.Sprint(day) + "日 周" + fmt.Sprint(value)
}

func DateMonthDayWeekday(month any, day any, wd any) string {
	return fmt.Sprint(month) + "月" + fmt.Sprint(day) + "日 " + fmt.Sprint(wd)
}

func DateMonthOf(value any) string {
	return fmt.Sprint(value) + "月"
}

func DateYearMonth(year any, month any) string {
	return fmt.Sprint(year) + "年" + fmt.Sprint(month) + "月"
}

func DateYearMonthDay(year any, month any, day any) string {
	return fmt.Sprint(year) + "年" + fmt.Sprint(month) + "月" + fmt.Sprint(day) + "日"
}

func DateYearMonthDayWeek(year any, month any, day any, value any) string {
	return fmt.Sprint(year) + "年" + fmt.Sprint(month) + "月" + fmt.Sprint(day) + "日 周" + fmt.Sprint(value)
}

const DiagAgentLog = "助手日志"

const DiagAgentNotRunning = "没有运行"

func DiagAgentRunningNode(value any) string {
	return "运行中 · Node.js " + fmt.Sprint(value)
}

const DiagAllGood = "组件与配置已找到"

const DiagChecking = "检查中"

const DiagCopied = "已复制诊断信息"

const DiagCopy = "复制诊断信息"

const DiagDaemon = "后台服务"

func DiagDaemonDetail(value any, port any) string {
	return "版本 " + fmt.Sprint(value) + " · 端口 " + fmt.Sprint(port)
}

const DiagDaemonLog = "后台日志"

const DiagFfmpegMissing = "未找到，请用系统的包管理器安装后重启后台"

const DiagFfmpegMissingWorkspace = "未安装，转写需要它"

const DiagFfmpegOk = "已找到 ffmpeg（还需配置识别渠道）"

func DiagIssues(issues any) string {
	return fmt.Sprint(issues) + " 项需要处理"
}

const DiagLogCopied = "已复制日志"

const DiagLogEmpty = "暂无记录"

func DiagLogLines(length any) string {
	return fmt.Sprint(length) + " 行"
}

const DiagLogNothing = "后台还没有写下任何记录"

func DiagLogRecent(length any) string {
	return "最近 " + fmt.Sprint(length) + " 行"
}

const DiagLogs = "日志"

const DiagMaintenance = "维护"

const DiagNotStarted = "没有启动"

const DiagOk = "正常"

func DiagReportLibrary(value any) string {
	return "课程库：" + fmt.Sprint(value)
}

func DiagReportLine(title any, detail any) string {
	return fmt.Sprint(title) + "：" + fmt.Sprint(detail)
}

const DiagRestart = "重启后台服务"

const DiagRestartAction = "重启"

const DiagRestartHint = "后台没有响应时使用；下载和转写会在重启后继续"

const DiagSetupRuntime = "设置运行环境"

const DiagStatus = "运行状态"

const DiagWorkspace = "Linux 工作区"

const DiagWorkspaceSuffix = " · Linux 工作区"

const DownloadAsrHint = "必剪 / 剪映免费识别，分段并行，带时间戳"

const DownloadChooseContent = "选择要下载的内容"

const DownloadDeleteLocalVideoAfterAsr = "已开启「转写完成后删除视频」：本地已有的视频会在转写完成后删除"

const DownloadDeleteVideoAfterAsr = "已开启「转写完成后删除视频」：视频会在文字稿生成后被删除"

func DownloadLectures(count any) string {
	return "下载 " + fmt.Sprint(count) + " 节课"
}

const DownloadNeedsFfmpeg = "请先在首页「功能准备」检查并安装 ffmpeg"

const DownloadNoCourseware = "平台不提供课件。在播放器的「资料 → 课件」里上传，MinerU 会解析成可检索的文字。"

func DownloadQueueFailed(e any) string {
	return "加入失败：" + fmt.Sprint(e)
}

func DownloadQueued(length any) string {
	return "已加入 " + fmt.Sprint(length) + " 个任务"
}

const DownloadSavedToLibrary = "文件保存在课程库里，按课程与日期整理"

const DownloadSlidesHint = "按时间戳命名，可与视频同步"

const DownloadStart = "开始下载"

const DownloadThisLecture = "下载这节课"

const DownloadVideoHint = "多路分段并发下载，可断点续传"

func DownloadsBulkDone(value any, n any) string {
	return fmt.Sprint(value) + " " + fmt.Sprint(n) + " 个任务"
}

const DownloadsCancelBody = "已下载的部分会保留，之后在课程里重新下载会从断点继续。"

const DownloadsCancelConfirm = "取消下载"

func DownloadsCancelTitle(n any) string {
	return "取消 " + fmt.Sprint(n) + " 个下载？"
}

const DownloadsClearAlsoFiles = "同时删除已下载的文件"

const DownloadsClearAlsoFilesHint = "视频、PPT、课件和文字稿会从磁盘移除，不可恢复"

const DownloadsClearAndDelete = "清除并删除文件"

const DownloadsClearBody = "任务记录会被清除。已下载的文件默认保留在课程库中。"

const DownloadsClearDone = "清除已完成"

func DownloadsClearFailed(e any) string {
	return "清除失败：" + fmt.Sprint(e)
}

const DownloadsClearRecords = "清除记录"

const DownloadsClearTitle = "清除已完成的任务"

const DownloadsCleared = "已清除任务记录"

func DownloadsClearedFreed(value any) string {
	return "已清除，释放 " + fmt.Sprint(value)
}

func DownloadsConnections(conns any) string {
	return fmt.Sprint(conns) + " 路并发"
}

const DownloadsDeleteAllFiles = "删除全部文件"

func DownloadsDeleteFilesBody(what any) string {
	return "将从磁盘删除" + fmt.Sprint(what) + "，不可恢复。需要时可以重新下载。"
}

const DownloadsDeleteFilesTitle = "删除文件？"

func DownloadsDeleteKindFiles(value any) string {
	return "删除" + fmt.Sprint(value) + "文件"
}

const DownloadsDeleteRecordAndFiles = "删除记录和文件"

func DownloadsDeletedFreed(value any) string {
	return "已删除，释放 " + fmt.Sprint(value)
}

const DownloadsEmpty = "还没有下载"

const DownloadsEmptyBody = "在课程里选择课次，点下载按钮。视频、PPT、课件和文字稿会按课程整理到课程库。"

const DownloadsExitSelection = "退出选择"

func DownloadsJobFiles(title any, value any, value2 any) string {
	return "「" + fmt.Sprint(title) + "」的" + fmt.Sprint(value) + "文件" + fmt.Sprint(value2)
}

const DownloadsKeep = "保留下载"

func DownloadsLectureAllFiles(value any) string {
	return "这节课的全部文件（" + fmt.Sprint(value) + "）"
}

func DownloadsLectureKind(value any, value2 any) string {
	return "这节课的" + fmt.Sprint(value) + "（" + fmt.Sprint(value2) + "）"
}

const DownloadsLocalEmpty = "本地还没有文件"

const DownloadsLocalEmptyBody = "下载完成的视频、PPT、课件和文字稿会出现在这里。"

const DownloadsNoneRunning = "没有进行中的任务"

const DownloadsPauseAll = "全部暂停"

func DownloadsPausedSummary(n any) string {
	return fmt.Sprint(n) + " 个任务已暂停"
}

func DownloadsPercent(value any) string {
	return fmt.Sprint(value) + "%"
}

const DownloadsRemoveRecordOnly = "仅移除记录"

const DownloadsResumeAll = "全部继续"

const DownloadsResumed = "已继续"

const DownloadsRetried = "已重试"

const DownloadsRetryAll = "全部重试"

func DownloadsRunningSummary(running any, value any) string {
	return fmt.Sprint(running) + " 个任务进行中 · " + fmt.Sprint(value)
}

const DownloadsSectionActive = "进行中"

const DownloadsSelectJobs = "选择任务"

const DownloadsSelectNone = "全不选"

func DownloadsSelectedCount(n any) string {
	return "已选 " + fmt.Sprint(n) + " 项"
}

func DownloadsSizeProgress(done any, total any) string {
	return fmt.Sprint(done) + " / " + fmt.Sprint(total)
}

func DownloadsSizeSuffix(value any) string {
	return "（" + fmt.Sprint(value) + "）"
}

func DownloadsStateRetrying(value any) string {
	return "网络中断，" + fmt.Sprint(value) + " 自动重试"
}

const DownloadsStateRunning = "下载中"

func DownloadsWaiting(length any) string {
	return fmt.Sprint(length) + " 个任务等待中"
}

func DurationDaysHours(inDays any, value any) string {
	return fmt.Sprint(inDays) + " 天 " + fmt.Sprint(value) + " 小时"
}

func DurationHours(h any) string {
	return fmt.Sprint(h) + " 小时"
}

func DurationHoursMinutes(h any, m any) string {
	return fmt.Sprint(h) + " 小时 " + fmt.Sprint(m) + " 分"
}

func DurationHoursMinutesOf(value any, value2 any) string {
	return fmt.Sprint(value) + " 小时 " + fmt.Sprint(value2) + " 分"
}

func DurationHoursMinutesShort(inHours any, value any) string {
	return fmt.Sprint(inHours) + " 小时 " + fmt.Sprint(value) + " 分"
}

func DurationHoursOnly(value any) string {
	return fmt.Sprint(value) + " 小时"
}

func DurationMinutes(m any) string {
	return fmt.Sprint(m) + " 分钟"
}

func DurationMinutesLong(inMinutes any) string {
	return fmt.Sprint(inMinutes) + " 分钟"
}

func DurationMinutesSeconds(value any, value2 any) string {
	return fmt.Sprint(value) + " 分 " + fmt.Sprint(value2) + " 秒"
}

func DurationSeconds(secs any) string {
	return fmt.Sprint(secs) + " 秒"
}

const DurationUnderMinute = "不到 1 分钟"

const EmojiAnimals = "动物"

const EmojiFaces = "表情"

const EmojiFood = "食物"

const EmojiHobbies = "爱好"

const EmojiNature = "自然"

const EmojiStudy = "学习"

const FolderSyncAccount = "NAS 账号"

const FolderSyncAccountHint = "和整库同步共用一个账号与目录，课程放在 courses/ 下"

const FolderSyncAutoHint = "保存后自动发布；接收到文件后自动刷新"

const FolderSyncBinaryCopy = "二进制文件，副本已完整保留。"

const FolderSyncBody = "把选中的课程直接同步到 NAS 的 WebDAV 目录。每台设备只发布改动过的文件，接收端合并并保留冲突副本；来源登录、模型密钥和本机设置留在本机。"

const FolderSyncCheck = "立即检查"

const FolderSyncConfirm = "连接并同步"

const FolderSyncConfirmTitle = "连接课程同步"

const FolderSyncConflicts = "冲突副本"

const FolderSyncConnect = "连接并同步"

const FolderSyncCourses = "同步的课程"

const FolderSyncKeepCurrent = "保留当前版"

func FolderSyncLastRun(up any, down any, conflicts any) string {
	return "已上传 " + fmt.Sprint(up) + " 项，已接收 " + fmt.Sprint(down) + " 项，冲突 " + fmt.Sprint(conflicts) + " 项。"
}

const FolderSyncLater = "稍后处理"

const FolderSyncNoConflicts = "没有待处理的冲突"

const FolderSyncNoCourses = "填好账号后点“刷新课程”，这里会列出本机和远端已有的课程。"

const FolderSyncOptions = "同步选项"

const FolderSyncOtherCopy = "保留的另一版本"

func FolderSyncPreview(files any, mb any, remote any, remoteMb any) string {
	return "本机 " + fmt.Sprint(files) + " 个文件（约 " + fmt.Sprint(mb) + " MB），远端 " + fmt.Sprint(remote) + " 个（约 " + fmt.Sprint(remoteMb) + " MB）。"
}

func FolderSyncPrivacy(media any) string {
	return "同步目录里的文件按原样保存，不做加密；视频音频" + fmt.Sprint(media) + "。请使用可信的 NAS 账号。"
}

const FolderSyncRefresh = "刷新课程"

func FolderSyncSelected(n any) string {
	return "已选 " + fmt.Sprint(n) + " 门课程。"
}

const FolderSyncSettingsGroup = "课程同步"

const FolderSyncSubtitle = "选择课程，把学习成果同步到 NAS"

const FolderSyncTitle = "课程同步"

const FolderSyncUseThis = "使用此副本"

const FolderSyncUsingCourseSync = "当前按课程同步；整库 WebDAV 同步尚未启用"

const FontHarmonySans = "鸿蒙黑体"

const FontInterWithSystem = "Inter + 系统中文"

const FontLxgwWenKai = "霞鹜文楷"

const FontSourceHanSans = "思源黑体"

const FontSystem = "系统默认"

const HeatmapLess = "少"

const HeatmapMore = "多"

const HomeAllCourses = "全部课程"

const HomeContinueWatching = "继续观看"

func HomeDate(month any, day any, wd any) string {
	return fmt.Sprint(month) + "月" + fmt.Sprint(day) + "日 · 周" + fmt.Sprint(wd)
}

const HomeDownloading = "正在下载"

const HomeFirstCourseBody = "从本机导入课堂录像，或连接学校的课程平台，就能开始看课、记笔记和复习。"

const HomeFirstCourseConnect = "连接课程平台"

const HomeFirstCourseLocal = "录像、转写和笔记都保存在本机"

const HomeFirstCourseTitle = "添加第一门课"

func HomeGreeting(greeting any) string {
	return fmt.Sprint(greeting) + "，"
}

const HomeGreetingAfternoon = "下午好"

const HomeGreetingEvening = "晚上好"

const HomeGreetingMorning = "早上好"

const HomeGreetingNight = "夜深了"

const HomeGreetingNoon = "中午好"

const HomeNextQuote = "换一句"

const HomeStatStorage = "占用空间"

const HomeStatistics = "统计"

const HomeThisTermCourses = "本学期课程"

const HomeUpNext = "接下来"

const KnowledgeAutoExportSuffix = " · 自动导出"

const KnowledgeConnectApps = "连接到笔记应用"

const KnowledgeCourseFolder = "课程文件夹"

func KnowledgeExported(value any) string {
	return "已导出 " + fmt.Sprint(value) + " 节课"
}

const KnowledgeFolderTemplates = "文件夹命名模板"

const KnowledgeFolderTemplatesHint = "课程可用 {title} {teacher} {term} {id}；课次可用 {date} {title} {sub_id} {lecturer}。只影响之后新建的文件夹。"

const KnowledgeLectureFolder = "课次文件夹"

const KnowledgeObsidianSubtitle = "导出到本地仓库，时间戳可点回 Coursive"

const KnowledgeSkillExport = "导出为课程笔记技能格式"

const KnowledgeSkillExportAction = "导出到技能目录"

const KnowledgeSkillExportHint = "以硬链接生成 raw/<课程>/<节次>/原始数据/ 目录，不额外占空间"

const KnowledgeTargetFolder = "目标文件夹"

const KnowledgeTargetFolderHelper = "课程笔记技能所在的文件夹"

const KnowledgeTemplatesSaved = "模板已保存"

const LectureNoReplay = "暂无回放"

const LectureNotStarted = "未开始"

func LectureOrdinal(ordinal any) string {
	return "第 " + fmt.Sprint(ordinal) + " 节"
}

const LevelHigh = "高"

const LevelMedium = "中"

const LevelStandard = "标准"

const LibraryAllCourses = "所有课程"

const LibraryLayoutHint = "每节课一个文件夹：video.mp4、slides/、courseware/、transcript.md/.srt、lecture.json。根目录的 README.md 说明了结构，方便 AI 工具直接阅读；library.db 是可重建的索引。"

func LibraryNotesCount(length any) string {
	return "笔记 · " + fmt.Sprint(length)
}

const LibraryNotesEmpty = "这门课还没有笔记。"

const LibraryNotesEmptyHint = "在播放器里边看边记，或让助手根据文字稿整理笔记。"

const LibraryReindex = "从文件重建索引"

func LibraryReindexFailed(e any) string {
	return "重建失败：" + fmt.Sprint(e)
}

func LibraryReindexed(value any) string {
	return "已重建索引，" + fmt.Sprint(value) + " 节课"
}

func LibrarySearchCount(length any) string {
	return fmt.Sprint(length) + " 条结果"
}

func LibrarySearchEmpty(value any) string {
	return "没有找到“" + fmt.Sprint(value) + "”"
}

const LibrarySearchEmptyBody = "只有转写过的课和笔记可以被搜索。"

const LibrarySearchHint = "例如：傅里叶变换、期中、作业"

const LibrarySearchTitle = "全文搜索所有文字稿与笔记"

const LibraryStatLocal = "节课在本地"

const LibraryStatSegments = "句文字稿"

const LibraryWriteNote = "写笔记"

const LinkCannotOpen = "这个链接无法打开"

func LinkCannotOpenFile(value any) string {
	return "无法打开 " + fmt.Sprint(value)
}

const LinkNotInLibrary = "这个文件不在课程库里"

const LinkRegister = "注册 coursive-app:// 链接"

func LinkRegisterFailed(e any) string {
	return "注册失败：" + fmt.Sprint(e)
}

func LinkRegisterPartial(name any, stderr any) string {
	return "已写入 " + fmt.Sprint(name) + "，但 xdg-mime 失败：" + fmt.Sprint(stderr)
}

const LinkRegistered = "已注册：点击 coursive-app:// 链接会打开 Coursive"

const ListComma = "，"

const ListSemicolon = "；"

const ListSeparator = "、"

const LmsAnnouncements = "通知"

func LmsAnnouncementsTab(count any) string {
	return "通知 " + fmt.Sprint(count)
}

func LmsAssignmentsTab(count any) string {
	return "作业 " + fmt.Sprint(count)
}

const LmsAutoLink = "自动匹配"

func LmsAutoMatch(title any) string {
	return "自动：" + fmt.Sprint(title)
}

const LmsAutoNoMatch = "自动：没有匹配的课程"

const LmsBadge = "学习平台"

func LmsCourseHeading(name any) string {
	return fmt.Sprint(name) + " · 学习平台"
}

const LmsCourseLinks = "课程对应"

const LmsCourseLinksBody = "把平台课程对应到资料库里的课程，作业和文件就会出现在课程页。"

const LmsDownloadToCourse = "下载到课程文件夹"

const LmsEmptyBody = "登录提供学习平台的来源后，作业、文件和通知会显示在这里。"

const LmsEmptyTitle = "没有可用的学习平台"

const LmsEntryHint = "学习平台的作业、文件和通知"

func LmsFileUpdated(month any, day any) string {
	return fmt.Sprint(month) + " 月 " + fmt.Sprint(day) + " 日更新"
}

const LmsFiles = "文件"

const LmsLinkCourse = "选择对应的课程"

const LmsLoadingCourses = "正在加载平台课程"

const LmsLoadingFiles = "正在加载文件"

const LmsNoAnnouncements = "没有通知"

const LmsNoAssignments = "没有作业"

const LmsNoAssignmentsBody = "平台发布作业后会显示在这里。"

const LmsNoCourses = "这个平台没有课程"

const LmsNoFiles = "没有文件"

const LmsNoFilesBody = "平台上传课程文件后会显示在这里。"

const LmsNoLink = "不对应"

const LmsNoOpenAssignments = "没有待交作业"

const LmsNotSynced = "尚未同步"

const LmsSavedToCourse = "已保存到课程文件夹"

const LmsSignedOut = "未登录"

func LmsSummary(courses any, open any) string {
	return fmt.Sprint(courses) + " 门课程 · " + fmt.Sprint(open) + " 项待交作业"
}

func LmsSyncFailed(error any) string {
	return "无法同步：" + fmt.Sprint(error)
}

const LmsTitle = "作业与通知"

const LocalCourse = "本地课程"

const LocalCourseCreate = "新建课程"

const LocalCourseCreateBody = "课程保存在本机。课堂录像可以现在导入，也可以稍后添加。"

func LocalCourseCreateFailed(error any) string {
	return "无法创建课程：" + fmt.Sprint(error)
}

const LocalCourseName = "课程名称"

const LocalCourseNameHint = "例如：线性代数"

const LocalCourseNameRequired = "请填写课程名称"

const LocalCourseSubmit = "创建课程"

func LocalCourseSubmitWithVideos(count int) string {
	return func() string {
		switch {
		}
		return "创建并导入 " + fmt.Sprint(count) + " 个视频"
	}()
}

const LocalCourseTeacher = "讲师或出处（选填）"

func LocalVideoImportFailed(name any, error any) string {
	return "无法导入“" + fmt.Sprint(name) + "”：" + fmt.Sprint(error)
}

const LocalVideoLimit = "超过 4 GiB，请移除后再导入"

const LocalVideosAdd = "添加视频"

const LocalVideosChoose = "选择视频"

const LocalVideosLater = "稍后添加"

const LocalVideosPartialSaved = "课程和已导入的课次都已保留。继续导入会从这个视频开始。"

func LocalVideosPickFailed(error any) string {
	return "无法选择视频：" + fmt.Sprint(error)
}

func LocalVideosProgress(done any, total any) string {
	return "已导入 " + fmt.Sprint(done) + "/" + fmt.Sprint(total)
}

const LocalVideosRechoose = "重新选择"

const LocalVideosResume = "继续导入"

const LocalVideosSection = "课堂录像"

const LocalVideosSectionHint = "视频按文件名顺序导入，每个视频成为一个课次。单个视频不超过 4 GiB。"

func LocalVideosSubmit(count int) string {
	return func() string {
		switch {
		case count == 0:
			return "导入视频"
		}
		return "导入 " + fmt.Sprint(count) + " 个视频"
	}()
}

func LocalVideosSummary(count any, size any) string {
	return fmt.Sprint(count) + " 个视频 · 共 " + fmt.Sprint(size)
}

const LoginSignIn = "登录"

const LoginSignInAgain = "重新登录"

const MarkdownImageUnavailable = "图片不可用"

const MemoryAboutYou = "关于你"

const MemoryAdd = "添加记忆"

const MemoryAppended = "追加"

const MemoryAuto = "自动沉淀"

const MemoryAutoHint = "对话停下两分钟、或攒够六轮后，在后台整理记忆"

const MemoryConsolidateNow = "立即沉淀"

const MemoryConsolidating = "正在沉淀"

func MemoryCount(length any) string {
	return "记得 " + fmt.Sprint(length) + " 件事"
}

func MemoryDeleteBody(title any) string {
	return "「" + fmt.Sprint(title) + "」会被忘掉，之后的对话不再用到。"
}

const MemoryDeleteTitle = "删除这条记忆？"

func MemoryEmpty(name any) string {
	return "还没有记忆。和 " + fmt.Sprint(name) + " 聊聊你的专业、学习习惯，或者直接说「记住……」。"
}

const MemoryEmptyShort = "还没有记忆"

const MemoryFileHint = "---\ntitle: 标题\ndescription: 一句话说明\n---\n内容"

const MemoryFileName = "文件名"

const MemoryFileNameInvalid = "名称只能用小写字母、数字、- 和 _"

const MemoryFromCourses = "从课程会话中学习"

const MemoryFromCoursesHint = "课程助手里的对话也参与沉淀"

func MemoryIntro(botName any) string {
	return fmt.Sprint(botName) + " 会把你说过的、反复用到的做法整理成记忆文件，每次对话都会带上「关于你」和「偏好」。这些文件只保存在本机，不会记下密码、证件号、银行卡号、手机号和密钥。"
}

func MemoryLastRun(value any) string {
	return "上次沉淀 " + fmt.Sprint(value)
}

const MemoryPreferences = "偏好"

func MemoryReadFailed(value any) string {
	return "读取记忆失败：" + fmt.Sprint(value)
}

const MemoryRoutines = "做法"

func MemoryRunChanged(length any) string {
	return "整理了 " + fmt.Sprint(length) + " 处"
}

const MemoryRunNothing = "没有需要记的"

func MemoryRunScope(value any, value2 any) string {
	return fmt.Sprint(value) + " 个对话 " + fmt.Sprint(value2) + " 轮"
}

const MemoryRunUnfinished = "没有完成"

const MemoryRuns = "沉淀记录"

const MemoryRunsEmpty = "还没有沉淀过。对话结束两分钟后会自动整理。"

const MemoryUpdated = "更新"

func MemoryWhen(when any) string {
	return "适用：" + fmt.Sprint(when)
}

const MemoryWorking = "正在整理记忆"

const MineruAutoParse = "上传后自动解析"

const MineruAutoParseHint = "关闭后需要在课件卡片上手动点「解析」"

const MineruChangeToken = "更换 Token"

const MineruForceOcr = "强制 OCR"

const MineruForceOcrHint = "扫描版或文字无法复制的 PDF 打开"

const MineruGetToken = "获取 Token"

const MineruIntro = "上传的课件（PDF、PPT、Word、图片）交给 MinerU 解析成 Markdown，保留公式、表格和图片，可以全文阅读，也能让助手引用。"

const MineruLite = "未配置 Token · 使用免登录的轻量解析"

const MineruLiteBody = "轻量解析不需要 Token，但单文件限 10 MB、20 页，输出不含图片。配置 Token 后使用精准解析（每天 1000 页高优先级额度）。"

const MineruModeLite = "轻量"

const MineruModePrecise = "精准"

const MineruModel = "解析模型"

const MineruPipelineHint = "Pipeline：传统版面分析，速度快、更稳定，适合排版简单的讲义。"

func MineruPrecise(value any) string {
	return "精准解析 · Token ••••" + fmt.Sprint(value)
}

func MineruQuotaBody(files any, daily any, value any, value2 any) string {
	return "今天解析了 " + fmt.Sprint(files) + " 个文件。高优先级额度每天 " + fmt.Sprint(daily) + " 页，超出后排队变慢；单文件上限 " + fmt.Sprint(value) + "、" + fmt.Sprint(value2) + " 页。额度由本机按解析结果统计，以官网为准。"
}

func MineruQuotaPages(daily any) string {
	return " / " + fmt.Sprint(daily) + " 页"
}

const MineruQuotaToday = "今日额度"

const MineruRecent = "最近解析"

func MineruTokenDaysLeft(expDays any) string {
	return "（还剩 " + fmt.Sprint(expDays) + " 天）"
}

func MineruTokenExpired(value any) string {
	return "Token 已于 " + fmt.Sprint(value) + " 过期，请重新生成"
}

func MineruTokenExpires(value any, value2 any) string {
	return "Token 有效期至 " + fmt.Sprint(value) + fmt.Sprint(value2)
}

const MineruTokenHint = "在 MinerU 官网「API 管理」中创建"

const MineruVlm = "VLM 精准"

const MineruVlmHint = "VLM：视觉大模型，公式、表格和复杂版面效果更好，推荐。"

const MirrorTuna = "清华大学"

const MirrorUbuntu = "Ubuntu 官方"

const MirrorUstc = "中国科学技术大学"

const ModelNone = "未选择模型"

func ModelTagContext(value any) string {
	return fmt.Sprint(value) + " 上下文"
}

const ModelTagDeprecated = "已弃用"

const ModelTagFree = "免费"

func ModelTagOutput(value any) string {
	return "输出 " + fmt.Sprint(value)
}

const ModelTagReasoning = "思考"

const ModelTagVision = "识图"

const ModelsAddProvider = "添加服务商"

const ModelsAssistantNotStarted = "助手还没启动"

const ModelsAssistantNotStartedBody = "启动后才能读取 Pi SDK 的模型注册表"

const ModelsAutoTitle = "自动为对话命名"

func ModelsAvailableCount(available any) string {
	return fmt.Sprint(available) + " 个可用模型"
}

const ModelsCatalogUpdated = "已更新 models.dev 目录"

func ModelsConfigError(value any) string {
	return "配置文件有误：" + fmt.Sprint(value)
}

const ModelsCurrent = "助手正在使用"

const ModelsImportPi = "导入 Pi CLI 配置"

func ModelsImported(value any, value2 any) string {
	return "导入了 " + fmt.Sprint(value) + " 个服务商、" + fmt.Sprint(value2) + " 个密钥"
}

const ModelsNoKey = "未填写密钥"

const ModelsNoProviders = "还没有服务商。点「添加服务商」，从 30 多家预设或 models.dev 目录里选择。"

const ModelsNoneAvailable = "没有可用的模型"

const ModelsOnboardingTitle = "给助手接上一个模型"

const ModelsPersona = "附加指令（可选）"

const ModelsPersonaHint = "例如：回答尽量简短；术语保留英文并附中文解释"

const ModelsPersonaSaved = "已保存附加指令"

const ModelsProviders = "已添加的服务商"

const ModelsSearch = "搜索模型"

const ModelsSearchAll = "搜索全部模型…"

const ModelsSettings = "模型设置…"

const ModelsStepKey = "填入 API Key"

const ModelsStepKeyHint = "只保存在本机，可一键测试连接"

const ModelsStepModels = "挑选模型"

const ModelsStepModelsHint = "从服务商拉取模型列表，自动补全价格和上下文长度"

const ModelsStepProvider = "选择服务商"

const ModelsStepProviderHint = "30+ 预设，或 models.dev 收录的 200 多家"

const ModelsUpdateCatalog = "更新 models.dev 目录"

const NavCourses = "课程列表"

const NavTasks = "定时"

func NoteAudioRef(value any) string {
	return "课堂原声 · " + fmt.Sprint(value)
}

const NoteByAssistant = "助手整理"

const NoteCopiedMarkdown = "已复制 Markdown"

const NoteCopyMarkdown = "复制 Markdown"

const NoteDeleteBody = "笔记文件会从课程库中删除。"

const NoteDeleteTitle = "删除这篇笔记？"

func NoteEditorHint(value any) string {
	return "支持 Markdown。" + fmt.Sprint(value) + "内容保存在课程库的 notes 文件夹。"
}

const NoteEditorHintTimestamp = "点“插入时间”记录当前播放位置，"

const NoteEmptyPreview = "_（空）_"

const NoteInsertTime = "插入时间"

const NoteMine = "我的笔记"

const NoteNew = "新笔记"

const NoteNoTranscriptHere = "这段没有文字稿"

const NoteOpenLecture = "打开对应课次"

func NotePlayFrom(value any) string {
	return "从 " + fmt.Sprint(value) + " 播放"
}

const NoteSaved = "笔记已保存"

func NoteSlideAt(idx any, value any) string {
	return "当时的 PPT · 第 " + fmt.Sprint(idx) + " 张，" + fmt.Sprint(value) + " 起"
}

func NoteSlideRef(slideIdx any, value any) string {
	return "PPT 第 " + fmt.Sprint(slideIdx) + " 张 · " + fmt.Sprint(value)
}

const NoteUnsaved = "未保存"

const NoteUntitled = "未命名笔记"

func NoteUpdated(value any) string {
	return "· " + fmt.Sprint(value) + "更新"
}

const NotesHint = "笔记以 Markdown 文件保存在课程库中，助手也能读取"

const NotificationsClose = "关闭通知"

const NotificationsEmpty = "暂时没有通知"

const NotificationsEmptyBody = "有新回放或新作业时会第一时间提醒你"

const NotificationsIntro = "新回放、新课次，以及学习平台的作业和通知会出现在这里。应用打开时在后台自动检查。"

const NotificationsMarkAllRead = "全部已读"

func NotificationsTooltip(unread any) string {
	return "通知（" + fmt.Sprint(unread) + " 条未读）"
}

const NotificationsUnread = "未读"

const NotionAutoSync = "笔记变化后自动同步"

const NotionAutoSyncHint = "保存或删除笔记约 1 分钟后，只同步改动的页面"

const NotionBody = "把课程库同步到 Notion 的一个页面下：每门课、每节课一个页面，下面是笔记和转写稿。只写入 Coursive 创建的页面；在 Notion 里改过的页面不会被覆盖。"

const NotionChangeParentBody = "之后会在新页面下重新创建课程库。原来同步过去的页面留在原处，不再更新。"

const NotionChangeParentTitle = "换一个同步位置？"

const NotionChoosePage = "选择页面"

const NotionChooseParent = "选择同步位置"

const NotionChooseParentHint = "这里只列出分享给集成的页面。也可以粘贴页面链接。"

func NotionConflicts(length any) string {
	return fmt.Sprint(length) + " 个页面在 Notion 里改过，保留了你的版本"
}

func NotionConnectedTo(value any) string {
	return "已连接到 " + fmt.Sprint(value)
}

const NotionCreateIntegration = "创建集成"

const NotionDisconnectBody = "会删除本机保存的令牌和同步记录。已经同步到 Notion 的页面会留在那里；之后重新连接时会新建一份。"

const NotionDisconnectTitle = "断开 Notion？"

func NotionErrors(length any) string {
	return fmt.Sprint(length) + " 个页面没有同步成功"
}

const NotionFullCheck = "完整检查"

const NotionFullCheckHint = "逐页确认还在，补建在 Notion 里删掉的页面；页面多时较慢"

const NotionIntegration = "集成"

const NotionLimits = "Notion 里的时间戳显示为文字，图片和课件不上传；需要跳回课堂时刻可以用 Obsidian 导出。"

func NotionMoreItems(value any) string {
	return "还有 " + fmt.Sprint(value) + " 项"
}

func NotionNoMatch(q any) string {
	return "没有找到「" + fmt.Sprint(q) + "」"
}

const NotionNoPages = "集成还没有可用的页面"

const NotionNoParent = "已连接 · 还没选同步位置"

const NotionOpenInNotion = "在 Notion 中打开"

const NotionOpenIntegrations = "打开 Notion 集成页"

const NotionParent = "同步位置"

const NotionParentHint = "课程库在这个页面下的「Coursive 课程库」里"

const NotionParentStep1 = "在 Notion 打开想放课程库的页面"

const NotionParentStep2 = "点右上角「···」→「连接」，选择你的集成"

const NotionParentStep3 = "回到这里选择这个页面"

const NotionSearchHint = "搜索页面或粘贴链接"

const NotionSecret = "集成密钥"

const NotionSecretHelper = "用本机密钥加密保存，只用来访问你授权给集成的页面"

func NotionSecretTail(value any) string {
	return "集成密钥 ••••" + fmt.Sprint(value)
}

const NotionShareHint = "连接后，还要在 Notion 里把一个页面分享给这个集成，课程库会建在那个页面下面。"

const NotionShareSteps = "在 Notion 打开目标页面，点右上角「···」→「连接」，选择你的集成，然后刷新。"

func NotionStatus(value any, value2 any, value3 any) string {
	return fmt.Sprint(value) + " · " + fmt.Sprint(value2) + " 个页面" + fmt.Sprint(value3)
}

const NotionStep1 = "在 Notion 的「集成」页新建一个 Internal 集成，名称填 Coursive，选择要同步到的工作区"

const NotionStep2 = "在「功能」里保留读取、更新和插入内容，其余可以关闭"

const NotionStep3 = "复制「Internal Integration Secret」，粘贴到下面"

const NotionSubtitle = "同步到你的 Notion 工作区"

const NotionUseLink = "使用这个链接"

const NotionWorkspace = "工作区"

const ObsidianAutoExport = "笔记变化后自动导出"

const ObsidianAutoExportHint = "保存或删除笔记后几秒内刷新"

const ObsidianBackLinks = "回到 Coursive"

const ObsidianBackLinksBody = "导出的时间戳和课次链接是 coursive-app:// 链接。注册后，在 Obsidian 里点击会打开对应的课和时刻。"

const ObsidianBody = "把课程库单向导出到 Obsidian 仓库的一个文件夹：总的「课程库」页，每门课一个文件夹，里面分「笔记」（按节次，含引用的 PPT 和课件）和「转写稿」。点击时间戳会回到 Coursive；在 Obsidian 里改过的文件不会被覆盖。"

const ObsidianChooseVaultFirst = "先选择 Obsidian 仓库文件夹"

func ObsidianConflicts(conflicts any) string {
	return fmt.Sprint(conflicts) + " 个文件在 Obsidian 中改过，已保留你的版本（笔记的新版本另存为「(Coursive)」副本）"
}

func ObsidianExported(value any, value2 any, value3 any) string {
	return "已导出：新写入 " + fmt.Sprint(value) + "，未变 " + fmt.Sprint(value2) + "，清理 " + fmt.Sprint(value3)
}

const ObsidianFolder = "仓库内的文件夹"

const ObsidianFolderHelper = "只写入这个文件夹，仓库里的其他笔记不受影响"

const ObsidianNoVault = "未设置仓库"

const ObsidianNotVault = "目标文件夹不是 Obsidian 仓库（没有 .obsidian），仍已导出"

const ObsidianSaveLocation = "保存位置"

func ObsidianStatus(value any, value2 any) string {
	return "导出到 " + fmt.Sprint(value) + fmt.Sprint(value2)
}

const ObsidianVault = "Obsidian 仓库"

const OutlineAccept = "采纳"

const OutlineAddItem = "添加条目"

const OutlineAppendix = "待核实"

const OutlineAppendixNote = "以下内容还没有经过你的确认，暂不作为结论。"

const OutlineAsk = "整理为复习提纲"

const OutlineAskHint = "确认资料后，助手把它们整理成草稿交给你审核"

const OutlineBody = "助手读完整份文字稿，按话题切成章节。点章节即可跳转，进度条上也会分段显示。"

const OutlineCardDraft = "草稿"

const OutlineCardSaved = "已存为笔记"

func OutlineCardSummary(sections int, items int) string {
	return fmt.Sprint(sections) + " 节 · " + fmt.Sprint(items) + " 条"
}

func OutlineChapterLine(start any, end any, title any) string {
	return fmt.Sprint(start) + "-" + fmt.Sprint(end) + "：" + fmt.Sprint(title)
}

const OutlineChecks = "待查"

const OutlineChoiceCustom = "自定义"

const OutlineChoiceKeep = "保持原样"

const OutlineChoiceKey = "重点"

const OutlineChoiceMastered = "已掌握"

const OutlineChoiceReview = "待复习"

const OutlineClose = "关闭审核，草稿会保留"

const OutlineConcise = "精简"

const OutlineConfirmBody = "助手会读取下面这些资料，整理成草稿交给你逐条审核。保存为课程笔记之前，不会写入任何内容。"

const OutlineConfirmStart = "开始整理"

const OutlineConfirmTitle = "整理为复习提纲"

const OutlineConflicts = "来源说法不一致"

const OutlineContents = "目录"

const OutlineCopied = "已复制大纲"

const OutlineCustomHint = "写下它的意思"

const OutlineDelete = "删除"

const OutlineDeleteSection = "删除章节，条目并入上一章"

const OutlineDeleted = "已删除大纲"

const OutlineDetailHint = "精简只分课堂的主要阶段，详细按知识点分章"

const OutlineDetailed = "详细"

const OutlineDirty = "有修改尚未更新到笔记"

const OutlineDropEvidence = "移除失效依据"

const OutlineEdit = "改写"

const OutlineEditHint = "Enter 完成 · Shift+Enter 换行 · Esc 放弃"

const OutlineEmpty = "没有可审核的条目"

const OutlineEvidenceGone = "依据已失效"

const OutlineExport = "导出"

func OutlineExportFailed(error string) string {
	return "无法导出：" + error
}

const OutlineExportMarkdown = "导出 Markdown"

const OutlineExportPdf = "导出 PDF"

func OutlineExportSaved(path string) string {
	return "已导出到 " + path
}

func OutlineFailed(e any) string {
	return "大纲生成失败：" + fmt.Sprint(e)
}

const OutlineGenerate = "生成大纲"

const OutlineImagesHint = "允许助手查看选区图像，用于读懂手写和扫描件"

const OutlineKeepAsCheck = "保留为待查"

const OutlineKeys = "A 采纳 · F 待查 · Enter 改写 · Alt+↑↓ 移动 · Esc 关闭"

const OutlineKindSummary = "归纳"

const OutlineLandscape = "横版"

func OutlineLastError(outlineError any) string {
	return "上次没有成功：" + fmt.Sprint(outlineError)
}

const OutlineLevel = "切换标题层级"

const OutlineMarginCompact = "紧凑"

const OutlineMarginStandard = "标准"

const OutlineMargins = "页边距"

const OutlineMarkHint = "这个标记是什么意思？只有你知道"

const OutlineMarkUndo = "撤销选择"

const OutlineMarks = "手写标记"

func OutlineMarksCount(n int) string {
	return fmt.Sprint(n) + " 个标记待解释"
}

const OutlineMerge = "合并下一条"

const OutlineMore = "更多操作"

const OutlineMoveDown = "下移"

const OutlineMoveUp = "上移"

const OutlineNeedsTranscript = "大纲需要文字稿"

const OutlineNeedsTranscriptBody = "先转写这节课，助手才能读完内容、按话题分章。"

const OutlineNextPage = "下一页"

const OutlineNoInk = "原来的笔迹已不在画布上"

const OutlineNoSources = "这个对话还没有绑定资料"

const OutlineOpenReview = "审核提纲"

const OutlineOpenSource = "在 Coursive 中打开"

const OutlineOptions = "大纲选项"

const OutlineOrientation = "方向"

const OutlinePaper = "纸张"

const OutlinePdfMarkdown = "改存 Markdown"

func OutlinePdfMeta(date string, sections int, items int, sources int) string {
	return date + " · " + fmt.Sprint(sections) + " 节 · " + fmt.Sprint(items) + " 条 · " + fmt.Sprint(sources) + " 处来源"
}

func OutlinePdfMissing(chars string, n int) string {
	return "无法导出 PDF：内嵌字体缺少「" + chars + "」等 " + fmt.Sprint(n) + " 个字符，可以改存 Markdown"
}

func OutlinePdfPage(page int, count int) string {
	return fmt.Sprint(page) + " / " + fmt.Sprint(count)
}

func OutlinePdfPreviewFailed(error string) string {
	return "无法预览：" + error + "。仍可保存同一份 PDF"
}

const OutlinePdfSave = "保存 PDF"

const OutlinePdfTextLayer = "文字可选择复制"

const OutlinePdfTitle = "导出 PDF"

const OutlinePeekOutline = "提纲"

const OutlinePeekSource = "原文"

const OutlinePortrait = "竖版"

const OutlinePrevPage = "上一页"

const OutlineQuestions = "未解决的问题"

const OutlineQuick = "整理提纲"

func OutlineRedone(mode any) string {
	return "已改为" + fmt.Sprint(mode) + "大纲"
}

const OutlineRegenerating = "正在重新分章…"

const OutlineRetry = "重试"

func OutlineReviewed(done int, total int) string {
	return "已审核 " + fmt.Sprint(done) + "/" + fmt.Sprint(total)
}

const OutlineSave = "保存为课程笔记"

const OutlineSaveConflict = "无法保存：这份笔记已在其他地方修改。本机的审核内容仍在，可以另存一份"

const OutlineSaveCopy = "另存为新笔记"

func OutlineSaveFailed(error string) string {
	return "无法保存笔记：" + error
}

const OutlineSaved = "已保存为本课笔记"

const OutlineSaving = "正在保存…"

const OutlineShowInk = "在画布上查看这处笔迹"

const OutlineSourceStale = "依据已失效"

const OutlineSources = "来源"

const OutlineSplit = "拆成两条"

func OutlineStaleCount(n int) string {
	return "依据失效 " + fmt.Sprint(n)
}

const OutlineStateNotRecorded = "无法记录提纲状态：笔记已保存，重试不会重复创建"

func OutlineSummary(length any, value any) string {
	return fmt.Sprint(length) + " 章 · " + fmt.Sprint(value)
}

const OutlineTagCheck = "待查"

const OutlineTagQuestion = "问题"

const OutlineTagUnconfirmed = "未确认"

const OutlineTagUnverified = "未核实"

const OutlineTitle = "给这节课分章"

const OutlineToCheck = "标为待查"

const OutlineTooManyItems = "每章最多 20 条"

const OutlineUnaccept = "取消采纳"

const OutlineUncheck = "取消待查"

const OutlineUndo = "撤销"

const OutlineUnread = "未读取的资料"

const OutlineUntitled = "复习提纲"

func OutlineUnverifiedCount(n int) string {
	return "未核实 " + fmt.Sprint(n)
}

const OutlineUpdate = "更新课程笔记"

const OutlineWorking = "正在通读文字稿并分章"

const OutlineWorkingHint = "长课可能需要一两分钟，可以先去看别的。"

const PanelAskNotes = "让助手整理笔记"

func PanelCoursewareCount(length any) string {
	return "课件 " + fmt.Sprint(length)
}

const PanelCurrentPosition = "当前位置"

const PanelMaterials = "资料"

const PanelQuiz = "出题"

const PanelQuizHint = "让助手根据这节课出一组选择题，每题附解析和出处"

const PanelQuizMore = "出题练习…"

func PanelSlidesCount(length any) string {
	return "PPT 截图 " + fmt.Sprint(length)
}

const PanelSummarizeRecent = "概括刚才这段"

const PickerChooseFolder = "选择文件夹"

const PickerFolderPath = "文件夹路径"

const PickerSaveToFolder = "保存到哪个文件夹"

func PlayerBookmarkAdded(value any) string {
	return "已添加书签 " + fmt.Sprint(value)
}

const PlayerBookmarkTooltip = "添加书签 (B)"

func PlayerBoosting(value any) string {
	return fmt.Sprint(value) + " 快进中"
}

const PlayerCollapsePanel = "收起侧栏"

const PlayerError = "播放出错"

const PlayerExitFullscreen = "退出全屏 (F)"

const PlayerExpandPanel = "展开侧栏"

const PlayerFullscreen = "全屏 (F)"

const PlayerLocalSuffix = "（本地）"

func PlayerMissingGstPlugins(packages any) string {
	return "系统的 GStreamer 缺少插件包 " + fmt.Sprint(packages) + "，用系统的包管理器装上后重启 Coursive 即可播放"
}

const PlayerMute = "静音 (M)"

const PlayerNoLectureAtTime = "这个时间点没有对应的课次"

const PlayerNoVideo = "这节课没有可播放的视频"

const PlayerPauseTooltip = "暂停 (空格)"

const PlayerPip = "画中画"

const PlayerPlayTooltip = "播放 (空格)"

const PlayerPreparing = "正在准备播放"

const PlayerReload = "重新加载"

func PlayerResumedFrom(value any) string {
	return "已从 " + fmt.Sprint(value) + " 继续"
}

func PlayerSeekBack(seekStepSec any) string {
	return "后退 " + fmt.Sprint(seekStepSec) + " 秒 (←)"
}

func PlayerSeekForward(seekStepSec any) string {
	return "前进 " + fmt.Sprint(seekStepSec) + " 秒 (→)"
}

func PlayerSeekToast(value any, s any) string {
	return fmt.Sprint(value) + fmt.Sprint(s) + " 秒"
}

func PlayerSpeedToast(value any) string {
	return "倍速 " + fmt.Sprint(value)
}

const PlayerSpeedTooltip = "倍速 ([ / ])"

const PlayerStartOver = "从头播放"

const PlayerSubtitlesTooltip = "字幕 (C)"

func PlayerVolumeToast(value any) string {
	return "音量 " + fmt.Sprint(value) + "%"
}

func PluginActionFailed(value any, value2 any) string {
	return "无法完成「" + fmt.Sprint(value) + "」：" + fmt.Sprint(value2)
}

const PluginCanvasFallbackEmpty = "这个对象没有可显示的文字"

const PluginCanvasFallbackExport = "导出数据"

const PluginCanvasFallbackMissing = "插件已不可用，以下内容仍保留在本机"

const PluginCanvasFallbackUnreadable = "无法显示这个插件对象，内容可能已损坏"

const PluginCanvasFallbackUntitled = "未命名的插件对象"

const PluginCategoryConnector = "连接器"

const PluginCategoryProgramming = "编程"

const PluginCategoryReview = "复习"

const PluginCategoryTutor = "学习方式"

const PluginCategoryVisualize = "可视化"

func PluginContextLine(value any, value2 any) string {
	return fmt.Sprint(value) + "：" + fmt.Sprint(value2)
}

func PluginError(pluginTitle any, value any) string {
	return fmt.Sprint(pluginTitle) + "：" + fmt.Sprint(value)
}

const PluginPages = "插件页面"

const PluginProposalApply = "批准写入"

func PluginProposalBody(value any) string {
	return "插件「" + fmt.Sprint(value) + "」要写入你本机的资料库。批准后才会写入，放弃则不做任何修改。"
}

const PluginProposalReject = "放弃"

func PluginProposalTitle(value any) string {
	return "允许「" + fmt.Sprint(value) + "」吗？"
}

func PluginRouteMissing(value any) string {
	return "无法打开「" + fmt.Sprint(value) + "」，插件没有提供这个页面"
}

const PluginSlotAssignment = "作业页"

const PluginSlotComposer = "输入框的「+」菜单"

const PluginSlotCourseMenu = "课程菜单"

const PluginSlotLectureMenu = "课次菜单"

const PluginSlotNoteToolbar = "笔记工具栏"

const PluginSlotScheduleItem = "日程条目"

const PluginViewAlternative = "换种方式看"

const PluginViewList = "清单"

func PluginViewUnknown(kind any) string {
	return "「" + fmt.Sprint(kind) + "」视图"
}

const PluginViewUnsupported = "当前版本的应用还不能显示这种视图，更新应用后即可查看"

const PluginWebAdmissionNeedsReview = "平台准入：评审中"

const PluginWebAdmissionSupported = "平台准入：支持"

const PluginWebAdmissionUnsupported = "平台准入：不支持"

func PluginWebExternalBody(value any) string {
	return "插件页面要打开 " + fmt.Sprint(value) + "。链接会在系统浏览器中打开，插件看不到之后的内容。"
}

const PluginWebExternalOpen = "打开链接"

func PluginWebExternalTitle(value any) string {
	return "在浏览器中打开「" + fmt.Sprint(value) + "」吗？"
}

const PluginWebLoading = "正在打开插件页面…"

const PluginWebNativeKept = "插件的原生页面、UI 扩展和动作照常可用，本地课程库与笔记不受影响。"

const PluginWebNativeKeptTitle = "仍然可用"

func PluginWebNavigationBlocked(value any) string {
	return "已阻止插件页面打开 " + fmt.Sprint(value)
}

func PluginWebOpenFailed(value any) string {
	return "无法打开插件页面：" + fmt.Sprint(value)
}

const PluginWebOpenNative = "打开原生页面"

const PluginWebPreview = "实验预览：这个 Web UI 渲染器还没有通过安全评审，只用于开发验证。"

const PluginWebReasonNeedsReview = "这个平台的自定义 Web UI 还在准入评审中，评审通过后才会开启。"

const PluginWebReasonPlatformUnsupported = "这个平台还不能运行插件的自定义 Web UI，插件页面改用原生渲染。"

const PluginWebReasonPluginInactive = "插件当前没有启用，启用后可以再试一次。"

const PluginWebReasonRendererMissing = "当前版本的应用还没有内置 WebView 渲染器，Web UI 页面暂时无法显示。"

const PluginWebReasonResourcesMissing = "插件的页面资源缺失或校验失败，重新安装插件后可以再试一次。"

const PluginWebReasonReviewPending = "这个平台的 Web UI 渲染器还没有通过单独的安全评审，评审通过前插件页面改用原生方式打开。"

const PluginWebUnavailableTitle = "这个插件页面暂时用原生方式打开"

const PluginsAfterAssistant = "助手启动后可用"

const PluginsAllowThirdParty = "允许第三方插件"

const PluginsAllowThirdPartyBody = "第三方插件不是 Coursive 发布的，与助手运行在同一进程中。只安装你信任的作者的插件；每次安装前都会列出它需要的权限。"

const PluginsAllowThirdPartyTitle = "允许第三方插件？"

const PluginsAppearsIn = "会出现在"

func PluginsAttentionCount(attention any) string {
	return fmt.Sprint(attention) + " 个需要处理"
}

func PluginsAttentionSuffix(attention any) string {
	return " · " + fmt.Sprint(attention) + " 个需要处理"
}

const PluginsBundled = "随应用提供"

const PluginsCapabilities = "助手获得的能力"

const PluginsCatalog = "插件目录"

func PluginsCatalogErrors(length any) string {
	return "有 " + fmt.Sprint(length) + " 个插件目录无法加载"
}

const PluginsCatalogHint = "owner/repo 或 https://…/coursive-catalog.json"

const PluginsCatalogs = "插件目录…"

const PluginsContribAlternative = "笔记的「换种方式看」"

func PluginsContribExport(value any) string {
	return fmt.Sprint(value) + "导出菜单"
}

const PluginsContribPage = "插件页面"

func PluginsCurrentVersion(value any) string {
	return "当前 v" + fmt.Sprint(value)
}

const PluginsDiscover = "发现插件"

const PluginsDiscoverTab = "发现"

const PluginsEmptyBody = "在「发现」里挑几个：换一种方式讲题、把笔记画成导图、连接你常用的工具。"

func PluginsEnabledCount(on any) string {
	return fmt.Sprint(on) + " 个启用"
}

func PluginsEnabledSummary(on any, value any) string {
	return fmt.Sprint(on) + " 个已启用" + fmt.Sprint(value)
}

const PluginsErrored = "出错"

const PluginsFeatured = "精选"

func PluginsFetchFailed(value any) string {
	return "无法获取插件：" + fmt.Sprint(value)
}

const PluginsFetching = "正在获取插件…"

const PluginsGoDiscover = "去发现"

const PluginsGuidanceOnly = "只在对话中提供指导与工具"

const PluginsHomepage = "主页"

func PluginsInstallFailed(value any) string {
	return "安装失败：" + fmt.Sprint(value)
}

const PluginsInstallFrom = "从文件夹、npm 或 Git 安装…"

const PluginsInstallOther = "从其他来源安装"

func PluginsInstallTitle(value any) string {
	return "安装「" + fmt.Sprint(value) + "」？"
}

const PluginsInstalled = "已安装的插件"

func PluginsInstalledCount(n any) string {
	return fmt.Sprint(n) + " 个已安装"
}

func PluginsInstalledLoadFailed(value any) string {
	return "已安装，但加载失败：" + fmt.Sprint(value)
}

func PluginsInstalledTab(n any) string {
	return "已安装 " + fmt.Sprint(n)
}

func PluginsInstalledToast(value any) string {
	return "已安装「" + fmt.Sprint(value) + "」"
}

const PluginsInstalling = "正在安装…"

func PluginsLoadFailed(value any) string {
	return "加载失败：" + fmt.Sprint(value)
}

const PluginsManage = "管理插件"

const PluginsMoreHint = "想要更多？允许第三方插件后，可以添加社区的插件目录，或从文件夹、npm、Git 安装。安装前都会列出它需要的权限。"

const PluginsNeedAssistant = "插件在助手里运行，助手启动后就能浏览和管理插件。"

const PluginsNeedsGrant = "需要授权"

const PluginsNewPermissions = "新增"

const PluginsNoMatch = "没有找到插件"

const PluginsNoMatchBody = "换个关键词，或在「来源」里添加更多插件目录"

const PluginsNoPermissions = "不需要额外权限"

const PluginsNoneInstalled = "还没有安装插件"

const PluginsOfficial = "Coursive 官方"

const PluginsOtherSource = "其他来源"

const PluginsPermissions = "权限"

const PluginsPinnedVersion = "需要写明固定版本"

const PluginsReviewGrant = "查看并授权"

const PluginsSaveSettings = "保存设置"

const PluginsSearch = "搜索插件"

const PluginsSettingNumber = "数字"

func PluginsSettingRange(value any, value2 any) string {
	return "应在 " + fmt.Sprint(value) + "–" + fmt.Sprint(value2) + " 之间"
}

func PluginsSettingRequired(value any) string {
	return "请输入" + fmt.Sprint(value)
}

const PluginsSettingText = "文字"

const PluginsSettingsSaved = "设置已保存"

func PluginsSource(value any) string {
	return "来源：" + fmt.Sprint(value)
}

const PluginsSources = "插件来源"

const PluginsSourcesTab = "来源"

const PluginsSubtitle = "让助手学会新本事：换一种方式讲题、把笔记画成导图、连接你常用的工具"

const PluginsTagline = "引导式提问、思维导图、闪卡复习……让助手学会新本事"

func PluginsThirdPartyWarning(value any) string {
	return "第三方插件，来源：" + fmt.Sprint(value) + "。它与助手运行在同一进程中，请确认你信任作者。"
}

const PluginsTryAfterEnable = "启用后可以这样问"

const PluginsTryAsking = "试试这样问"

const PluginsUninstallBody = "插件保存的数据会一起删除。已经生成的笔记、练习等内容不受影响。"

func PluginsUninstallTitle(value any) string {
	return "卸载「" + fmt.Sprint(value) + "」？"
}

const PluginsUpdatable = "可更新"

func PluginsUpdateTitle(value any) string {
	return "更新「" + fmt.Sprint(value) + "」？"
}

func PluginsUpdatesCount(updates any) string {
	return fmt.Sprint(updates) + " 个可更新"
}

func PluginsVersionAvailable(value any) string {
	return "v" + fmt.Sprint(value) + " 可用"
}

const PluginsWillBeAble = "它将可以"

func PracticeAccuracy(value any) string {
	return "正确率 " + fmt.Sprint(value) + "%"
}

const PracticeAskWhyRight = "请讲讲背后的原理，帮我巩固。"

const PracticeAskWhyWrong = "请解释为什么错，以及我可能混淆了什么。"

const PracticeBackToSummary = "返回总结"

func PracticeBest(label any) string {
	return "最好成绩 " + fmt.Sprint(label)
}

const PracticeByTopic = "按知识点"

const PracticeCorrect = "答对"

const PracticeCorrectAnswer = "正确答案"

const PracticeDone = "练习完成"

const PracticeExplain = "讲解"

const PracticeHint = "提示"

const PracticeMoreOnWrong = "再出一组（针对错题）"

const PracticeNext = "下一题"

func PracticeOption(value any, text any) string {
	return fmt.Sprint(value) + "「" + fmt.Sprint(text) + "」"
}

func PracticeOptionLabel(letter any, value any) string {
	return "选项 " + fmt.Sprint(letter) + fmt.Sprint(value)
}

func PracticeOptionNote(note any) string {
	return "，" + fmt.Sprint(note)
}

const PracticeOptionSeparator = "；"

const PracticePrevious = "上一题"

func PracticeQuestionNumber(number any) string {
	return "第 " + fmt.Sprint(number) + " 题"
}

const PracticeRetryDone = "重做完成"

func PracticeRetryTitle(title any) string {
	return "重做错题 · " + fmt.Sprint(title)
}

func PracticeRetryWrong(length any) string {
	return "重做错题 " + fmt.Sprint(length)
}

const PracticeReview = "逐题回顾"

func PracticeScoreNotSaved(e any) string {
	return "成绩没有保存：" + fmt.Sprint(e)
}

func PracticeScoreSemantics(correct any, length any) string {
	return "答对 " + fmt.Sprint(correct) + " 题，共 " + fmt.Sprint(length) + " 题"
}

func PracticeSecondsSuffix(value any) string {
	return " " + fmt.Sprint(value) + " 秒"
}

const PracticeSource = "查看出处"

func PracticeSourceRef(ref any) string {
	return "出处：" + fmt.Sprint(ref)
}

func PracticeTime(time any) string {
	return "用时 " + fmt.Sprint(time)
}

const PracticeUnanswered = "未作答"

const PracticeWrong = "答错"

const PracticeYourChoice = "你的选择"

const ProfileBackground = "底色"

func ProfileBackgroundIndex(index any) string {
	return "底色 " + fmt.Sprint(index)
}

const ProfileChangeAvatar = "更换头像"

const ProfileEdit = "编辑个人资料"

const ProfileEditTooltip = "编辑昵称和头像"

const ProfileEmojiHint = "输入或粘贴任意表情"

const ProfileFromFile = "从图片文件…"

const ProfileFromQq = "使用 QQ 头像…"

const ProfileFromUrl = "输入图片链接…"

const ProfileImageAddress = "图片地址"

func ProfileImageFailed(e any) string {
	return "无法使用这张图片：" + fmt.Sprint(e)
}

const ProfileImageUrl = "图片链接"

const ProfileInspiration = "灵感"

const ProfileLocalOnly = "昵称和头像只保存在本机"

const ProfileNickname = "昵称"

func ProfileNicknameHelper(fallback any) string {
	return "留空则显示「" + fmt.Sprint(fallback) + "」"
}

const ProfilePickEmoji = "选择表情"

const ProfileQqHint = "5–12 位数字"

const ProfileQqInvalid = "请输入 5–12 位 QQ 号"

const ProfileQqNote = "头像从 QQ 公开头像服务加载，需要联网"

const ProfileQqNumber = "QQ 号"

const ProfileRandomShape = "随机生成图形"

const ProfileResetInitial = "重置为首字"

const ProfileShape = "轮廓"

const ProfileShowQuote = "首页显示一言"

const ProfileShowQuoteHint = "欢迎语下方展示一句文学、诗词或哲思"

const ProfileShuffle = "换一批"

const ProfileTapToUse = "点一下换上"

const ProfileTitle = "个人资料"

const ProfileUrlInvalid = "需要 http(s) 链接"

const ProfileUseQq = "使用 QQ 头像"

func PromptCreateQuiz(lectureId any, count any, value any, value2 any) string {
	return "请为这节课（lecture_id " + fmt.Sprint(lectureId) + "）出 " + fmt.Sprint(count) + " 道" + fmt.Sprint(value) + "难度的选择题，保存为练习" + fmt.Sprint(value2) + "。"
}

func PromptExplainQuestion(title any, value any, stem any, value2 any, value3 any, value4 any, value5 any, value6 any) string {
	return "讲解练习「" + fmt.Sprint(title) + "」第 " + fmt.Sprint(value) + " 题：" + fmt.Sprint(stem) + "\n选项：" + fmt.Sprint(value2) + "\n我选了 " + fmt.Sprint(value3) + "，正确答案是 " + fmt.Sprint(value4) + "。" + fmt.Sprint(value5) + fmt.Sprint(value6)
}

const PromptNotes = "请为这节课整理一份完整的学习笔记并保存：以字幕为主，找出重点和不确定的地方，对照当时的 PPT 和已上传的课件核实，公式用 LaTeX，要点附时间戳。"

func PromptRetryWrong(title any, id any) string {
	return "根据我在练习「" + fmt.Sprint(title) + "」（id " + fmt.Sprint(id) + "）中做错的题，针对这些知识点再出一组练习题。"
}

const PromptSummarizeRecent = "用几句话概括我刚才看的这一段（当前播放位置前 5 分钟）讲了什么。"

const ProposalApplied = "已应用"

const ProposalExpired = "已过期，请让助手重新生成"

const ProposalFailed = "执行失败"

const ProposalIgnored = "已忽略"

const ProposalPending = "等你确认"

func ProviderAddTitle(name any) string {
	return "添加 " + fmt.Sprint(name)
}

const ProviderAnthropicBlurb = "Claude 系列"

const ProviderApi = "接口协议"

const ProviderBailian = "阿里云百炼"

const ProviderBailianBlurb = "通义千问 Qwen 全系列"

const ProviderBuiltIn = "Pi SDK 内置服务商"

const ProviderBuiltInHint = "Pi SDK 已内置这家的模型目录，填好密钥即可使用。点模型可设为助手默认。"

const ProviderCatalogNote = "价格（美元 / 百万 token）、上下文长度和能力标签来自 models.dev，保存时写入 Pi 的模型配置，用于上下文管理和用量统计。"

const ProviderChosen = "已选择"

func ProviderChosenModels(length any) string {
	return "已选 " + fmt.Sprint(length) + " 个模型 · 保存后测试连接"
}

const ProviderCustom = "自定义服务"

const ProviderDeepseekBlurb = "深度求索，性价比高的推理与对话模型"

const ProviderDeleteBody = "会删除这个服务及其模型配置。"

func ProviderDeleteTitle(name any) string {
	return "删除 " + fmt.Sprint(name) + "？"
}

func ProviderDownloadApp(name any) string {
	return "下载 " + fmt.Sprint(name)
}

const ProviderFetchModels = "从服务商获取模型列表"

const ProviderFetchNeedsKey = "填写密钥后获取模型列表"

const ProviderFreeTrialBlurb = "免费试用额度"

const ProviderFromCatalog = "从 models.dev 选择"

func ProviderGetKey(name any) string {
	return "还没有密钥？去 " + fmt.Sprint(name) + " 获取"
}

const ProviderGoogleBlurb = "Gemini 系列，AI Studio 密钥"

const ProviderGroqBlurb = "LPU 极速推理"

const ProviderHunyuan = "腾讯混元"

const ProviderHunyuanBlurb = "腾讯混元大模型"

const ProviderId = "服务 ID"

const ProviderIdHelper = "本地标识，字母数字及 . _ -"

const ProviderKeyHint = "粘贴你的密钥"

const ProviderKeyMissing = "请先填写 API Key"

func ProviderKeySaved(value any) string {
	return "已保存（" + fmt.Sprint(value) + "），输入新值以替换"
}

const ProviderLmstudioBlurb = "图形化的本地模型服务"

const ProviderLocalNoKey = "本地服务通常不需要密钥。确认服务已在本机启动。"

const ProviderManualModel = "手动添加模型 ID"

const ProviderManualModelHint = "例如 deepseek-chat"

const ProviderMimo = "小米 MiMo"

const ProviderMimoBlurb = "小米 MiMo 推理模型"

const ProviderMinimaxBlurb = "稀宇科技 MiniMax 模型"

const ProviderModelscope = "魔搭 ModelScope"

const ProviderModelscopeBlurb = "每日免费额度的开源模型推理"

const ProviderMoonshot = "Kimi 月之暗面"

const ProviderMoonshotBlurb = "长上下文的 Kimi 模型"

func ProviderMoreModels(value any) string {
	return "还有 " + fmt.Sprint(value) + " 个，搜索以缩小范围"
}

const ProviderNoMatch = "没有匹配的模型"

const ProviderNoModelList = "这家服务可能不提供模型列表接口，可以改从 models.dev 目录选择或手动添加。"

const ProviderNotInCatalog = "models.dev 未收录"

func ProviderOfferedCount(length any, offeredFrom any) string {
	return fmt.Sprint(length) + " 个 · 来自" + fmt.Sprint(offeredFrom)
}

const ProviderOllamaBlurb = "在本机运行开源模型"

const ProviderOneKeyBlurb = "一个 Key 调用多家模型"

const ProviderOpenaiBlurb = "GPT 系列"

const ProviderOpenrouterBlurb = "数百个模型的统一入口"

const ProviderPayAsYouGoBlurb = "按量付费的模型聚合平台"

const ProviderPerplexityBlurb = "联网搜索的 Sonar 模型"

const ProviderPickModel = "至少选择一个模型"

const ProviderPpio = "PPIO 派欧云"

const ProviderPpioBlurb = "分布式云上的开源模型 API"

const ProviderQianfan = "百度千帆"

const ProviderQianfanBlurb = "文心 ERNIE 系列"

const ProviderRemoveKeyBody = "内置服务商仍会保留在目录中，填入密钥即可重新使用。"

func ProviderRemoveKeyTitle(name any) string {
	return "移除 " + fmt.Sprint(name) + " 的密钥？"
}

const ProviderSaveAndTest = "保存并测试"

func ProviderSaved(name any) string {
	return "已保存 " + fmt.Sprint(name)
}

const ProviderSiliconflow = "硅基流动"

const ProviderSiliconflowBlurb = "聚合 Qwen、DeepSeek、GLM 等开源模型"

const ProviderSource = "服务商"

const ProviderStepfun = "阶跃星辰"

const ProviderStepfunBlurb = "Step 系列多模态模型"

func ProviderTestOk(value any, value2 any) string {
	return fmt.Sprint(value) + " 响应正常 · " + fmt.Sprint(value2) + " ms"
}

const ProviderTestsOnSave = "保存后会测试一次连接"

const ProviderVolcengine = "火山方舟 · 豆包"

const ProviderVolcengineBlurb = "字节跳动豆包大模型"

const ProviderXaiBlurb = "Grok 系列"

const ProviderZhipu = "智谱 BigModel"

const ProviderZhipuBlurb = "GLM 系列"

const QuizAgain = "再练一次"

const QuizAssistantBusy = "助手正在回复，请稍后再试"

const QuizAssistantNotReady = "助手还没有就绪"

func QuizBestAttempts(label any, attempts any) string {
	return "最好 " + fmt.Sprint(label) + " · 练过 " + fmt.Sprint(attempts) + " 次"
}

func QuizCount(count any) string {
	return fmt.Sprint(count) + " 题"
}

const QuizCreate = "让助手出题"

const QuizCreateHint = "助手会先读完字幕并核对 PPT 与课件，每道题都附出处和解析。"

const QuizCreateTitle = "出题练习"

const QuizDifficulty = "难度"

const QuizEasy = "简单"

const QuizFocus = "侧重（可选）"

const QuizFocusAll = "，覆盖整节课的重点"

const QuizFocusHint = "例如：卷积的性质、例题的解法"

func QuizFocusOn(focus any) string {
	return "，侧重：" + fmt.Sprint(focus)
}

const QuizHard = "困难"

const QuizMedium = "中等"

const QuizNone = "还没有练习"

func QuizQuestions(n any) string {
	return fmt.Sprint(n) + " 题"
}

const QuizSize = "题量"

const QuizStart = "开始练习"

const Range30Days = "30 天"

const Range7Days = "7 天"

const Range90Days = "90 天"

const RangeThisYear = "今年"

const ReaderFindHint = "在本文中查找"

const ReaderFullscreen = "全屏阅读"

const ReaderNextMatch = "下一处"

const ReaderNoHeadings = "这篇没有小标题"

const ReaderNoMatches = "无结果"

const ReaderOpen = "在阅读器中打开"

const ReaderPreviousMatch = "上一处"

const RegionChina = "国内"

const RegionGlobal = "海外"

const RestoreMerge = "合并"

const RestoreMergeHint = "补上本机没有的文件，更新较旧的文件；本机较新的文件保持不变。"

const RestoreNew = "本机没有"

const RestoreNewer = "备份里的较新"

const RestoreOlder = "本机的较新"

const RestoreOverwrite = "以备份为准"

const RestoreOverwriteHint = "所有文件换成备份里的版本；被替换的较新文件会移到课程库的 .sync-conflicts 文件夹。"

const RestoreSame = "相同"

const RestoreSkipped = "不会恢复（密钥或无效路径）"

const ReviewCancel = "取消审校"

const ReviewCancelled = "已取消，保留已检查部分的旁注。"

func ReviewCompleted(count int) string {
	return "整节课已审校 · " + fmt.Sprint(count) + " 条推测，供核对"
}

const ReviewConsent = "将这节课的完整文字稿、课程信息，以及可用的该节课堂 OCR / 课件文字发送给已配置的模型服务。按段检查可能产生模型费用。结果以「AI 推测」旁注显示，原文、字幕和时间戳保持不变；模型推测需要你核对。未完成的同版本审校会从已保存进度继续；重新审校已完成稿会替换已有旁注。"

func ReviewDetails(original string, reason string, evidence string) string {
	return "原文：" + original + "\n\n推测理由：" + reason + "\n\n依据：" + evidence + "\n\n这是模型推测，请结合录音核对。"
}

const ReviewDismiss = "忽略这条建议"

const ReviewFailed = "审校未完成，请检查模型服务后重试；已保存部分结果。"

func ReviewFiltered(count int) string {
	return "已过滤 " + fmt.Sprint(count) + " 条无法匹配原文或依据的候选建议。"
}

func ReviewGuess(guess string) string {
	return "（AI 推测：" + guess + "）"
}

const ReviewHint = "保留原文，自动检查整节课的疑似识别错误。"

const ReviewInterrupted = "审校进程已停止，可以重新审校。"

func ReviewLoadFailed(error string) string {
	return "无法读取或更新审校结果：" + error
}

func ReviewModel(model string) string {
	return "审校模型：" + model
}

func ReviewProgress(done any, total any) string {
	return "正在审校：" + fmt.Sprint(done) + " / " + fmt.Sprint(total) + " 段"
}

const ReviewShow = "显示 AI 旁注"

const ReviewStale = "文字稿已变化，这份旁注已过期。请重新审校。"

const ReviewStart = "AI 审校"

const RunAborted = "已中止"

const RunCostLimit = "超出费用上限"

const RunMissed = "已错过"

const RuntimeDeps = "助手依赖（Pi SDK）"

const RuntimeInstall = "开始安装"

const RuntimeInstallDeps = "安装依赖"

const RuntimeInstallLog = "安装日志"

const RuntimeInstallTitle = "安装助手运行环境"

const RuntimeInstalling = "正在安装"

const RuntimeIntro = "助手运行在 Node.js 上，并使用 Pi SDK。缺少的部分会安装到 Coursive 的数据目录，不改动系统；安装包会按官方发布的 SHA-256 校验。"

const RuntimeMirrorChina = "国内镜像"

const RuntimeMirrorOfficial = "官方源"

const RuntimeNodeMissing = "未找到可用的 Node.js 22+"

func RuntimeReadFailed(error any) string {
	return "无法读取运行环境：" + fmt.Sprint(error)
}

const RuntimeReady = "运行环境已就绪"

const RuntimeRecheck = "重新检查"

const RuntimeSourceManaged = "应用内安装"

const RuntimeSourceSystem = "系统"

const RuntimeStartAssistant = "启动助手"

const RuntimeStarting = "正在启动…"

const RuntimeStartsAfterInstall = "安装完成后自动启动"

const ScheduleAdd = "添加日程"

const ScheduleAllDay = "全天"

const ScheduleAlreadyAdded = "日程里已有"

const ScheduleCollapseMonth = "收起月历"

func ScheduleDaily(value any) string {
	return "每天 " + fmt.Sprint(value)
}

const ScheduleDayEmpty = "这天没有安排"

func ScheduleDayItems(length any) string {
	return "，" + fmt.Sprint(length) + " 项日程"
}

func ScheduleDayLabel(month any, day any, value any) string {
	return fmt.Sprint(month) + "月" + fmt.Sprint(day) + "日" + fmt.Sprint(value)
}

func ScheduleDaysLeft(diff any) string {
	return "还有 " + fmt.Sprint(diff) + " 天"
}

const ScheduleDueDate = "截止日期"

const ScheduleEdit = "编辑日程"

const ScheduleEmpty = "还没有日程"

const ScheduleEmptyBody = "老师在课上说到带日期的作业、考试或活动时，助手可以把它标记进来，并附上原话和录像时间点。也可以自己添加。"

const ScheduleExpandMonth = "展开月历"

const ScheduleHiddenFromSchedule = "已从日程隐藏"

const ScheduleHide = "从日程隐藏"

const ScheduleJumpToSource = "跳到老师提到的地方"

const ScheduleKindDeadline = "截止"

const ScheduleKindEvent = "活动"

const ScheduleKindExam = "考试"

const ScheduleMarkUndone = "标为未完成"

const ScheduleNew = "新建日程"

const ScheduleNextMonth = "下个月"

const ScheduleNextWeek = "下一周"

const ScheduleNoCourse = "不关联课程"

const ScheduleNoTodos = "没有待办"

const ScheduleNote = "备注"

const ScheduleNoteHint = "要求、提交方式…"

const ScheduleOnce = "一次"

func ScheduleOnceAt(month any, day any, value any) string {
	return fmt.Sprint(month) + "月" + fmt.Sprint(day) + "日 " + fmt.Sprint(value) + "（一次）"
}

const ScheduleOpenOnPlatform = "在学习平台打开"

func ScheduleOpenTodos(open any) string {
	return fmt.Sprint(open) + " 项待办"
}

func ScheduleOverdue(overdue any) string {
	return fmt.Sprint(overdue) + " 项已逾期"
}

func ScheduleOverdueDays(value any) string {
	return "逾期 " + fmt.Sprint(value) + " 天"
}

const ScheduleOverdueHeader = "已逾期"

const SchedulePrevMonth = "上个月"

const SchedulePrevWeek = "上一周"

func ScheduleRemovedNotice(value any, title any) string {
	return fmt.Sprint(value) + "「" + fmt.Sprint(title) + "」"
}

const ScheduleTitleHint = "例如：实验二报告"

const ScheduleTodos = "待办"

func ScheduleWeekly(value any, value2 any) string {
	return "每周" + fmt.Sprint(value) + " " + fmt.Sprint(value2)
}

func ScheduleWorkdays(value any) string {
	return "工作日 " + fmt.Sprint(value)
}

const SchemeContent = "内容"

const SchemeExpressive = "表现"

const SchemeFidelity = "忠实"

const SchemeMonochrome = "单色"

const SchemeNeutral = "中性"

const SchemeTonalSpot = "柔和"

const SchemeVibrant = "鲜明"

const ScreenCourse = "课程详情"

const ScreenDownloads = "下载管理"

const ScreenHome = "首页"

const ScreenLibrary = "资料库"

const ScreenPlayer = "播放器"

const ScrollLeft = "向左滚动"

const ScrollRight = "向右滚动"

func SearchFailed(e any) string {
	return "搜索失败：" + fmt.Sprint(e)
}

const SettingsAbout = "关于"

const SettingsAboutHint = "版本、更新和反馈"

const SettingsAccount = "账户与来源"

const SettingsAccountHint = "个人资料、来源插件"

const SettingsAppearance = "外观与播放"

const SettingsAppearanceGroup = "外观"

const SettingsAppearanceHint = "主题、字体、播放器"

const SettingsAsrChannels = "转写渠道"

const SettingsAssistant = "助手与插件"

const SettingsAssistantHint = "模型、Bot、插件"

const SettingsBoostSpeed = "长按快进倍速"

const SettingsBoostSpeedHint = "长按画面或按住 → 键临时加速"

const SettingsBotHint = "常驻 bot 的形象、记忆与学习方式"

const SettingsBotKeyword = "Bot 形象与记忆"

const SettingsBubbles = "回复使用气泡"

const SettingsBubblesKeyword = "回复气泡"

const SettingsChatTextSize = "对话文字大小"

const SettingsCleanCache = "清理缓存"

const SettingsCompact = "紧凑布局"

const SettingsConnections = "单个视频的并发连接"

const SettingsConnectionsHint = "分段并行下载，支持断点续传；校园网瓶颈时增加连接帮助有限"

const SettingsConnectionsKeyword = "并发连接"

func SettingsConnectionsValue(value any) string {
	return fmt.Sprint(value) + " 路"
}

const SettingsContrast = "对比度"

const SettingsDaemonKeyword = "后台服务"

const SettingsDarkMode = "深色模式"

const SettingsData = "数据与同步"

const SettingsDataHint = "WebDAV 同步、备份恢复、存储空间"

const SettingsDefaultSpeed = "默认倍速"

const SettingsDiagnostics = "诊断"

const SettingsDiagnosticsHint = "后台状态与日志"

const SettingsDock = "宽屏时常驻在右侧"

const SettingsDockKeyword = "宽屏常驻右侧"

const SettingsFont = "字体"

const SettingsGlass = "液态玻璃"

const SettingsGlassHighContrast = "高对比度下使用实色"

const SettingsGlassHint = "画布工具栏与输入框透出下方内容；关闭后改为实色"

const SettingsKnowledge = "知识库"

const SettingsKnowledgeHint = "课程库、Obsidian、Notion"

const SettingsLibraryLocation = "课程库位置"

const SettingsLocalModels = "本地模型"

const SettingsMineru = "MinerU 课件解析"

const SettingsModels = "模型与服务商"

func SettingsNoMatch(value any) string {
	return "没有和「" + fmt.Sprint(value) + "」相关的设置"
}

const SettingsNotificationsHint = "新回放、新课次与自动检查"

const SettingsProfileKeyword = "昵称和头像"

const SettingsProfileLocal = "资料只保存在本机"

const SettingsRateLimit = "总速度上限"

const SettingsReduceMotion = "减少动画"

const SettingsRemindersKeyword = "提醒"

const SettingsRestartKeyword = "重启后台服务"

const SettingsResume = "从上次位置继续"

const SettingsRuntime = "运行环境"

const SettingsRuntimeKeyword = "运行环境 Node.js"

const SettingsRuntimeMissing = "助手未运行 · 检查或安装 Node.js 与 Pi SDK"

const SettingsRuntimeReady = "Node.js 与 Pi SDK 已就绪"

const SettingsSchemeStyle = "配色风格"

const SettingsSearch = "搜索设置"

const SettingsSeekStep = "快进 / 快退步长"

const SettingsSeekStepKeyword = "快进快退步长"

const SettingsShortcuts = "快捷键"

const SettingsShortcutsBody = "快捷键：空格 播放/暂停 · ←/→ 快退/快进 · ↑/↓ 音量 · [ ] 倍速 · F 全屏 · C 字幕 · B 书签 · P 侧栏 · M 静音"

const SettingsShowSubtitles = "显示字幕"

const SettingsShowSubtitlesHint = "有文字稿时显示字幕"

const SettingsShowThinking = "显示思考过程"

const SettingsSkillExport = "技能格式导出"

const SettingsSlidesFollow = "PPT 跟随播放"

const SettingsSlidesFollowHint = "PPT 跟随播放进度"

const SettingsStream = "下载哪个画面"

const SettingsStreamAnimation = "流式输出动画"

const SettingsStreamHint = "自动：优先屏幕（含 PPT），没有时用第一个画面。"

const SettingsSubtitleSize = "字幕大小"

const SettingsSyncMedia = "同步视频"

const SettingsSystemAccent = "系统强调色"

const SettingsTasksHint = "让助手按计划复盘、出题、汇总截止事项"

const SettingsTextSize = "文字大小"

const SettingsThemeColor = "主题色"

const SettingsTransfer = "下载与转写"

const SettingsTransferHint = "下载速度、语音转写、课件解析"

const SettingsUsage = "用量与费用"

const SettingsUsageHint = "按模型、用途和对话查看 token 与估算费用"

const SettingsUseSystemAccent = "使用系统强调色"

const SettingsUseSystemAccentHint = "从系统主题取色生成配色"

const SettingsVideoParallel = "同时下载的视频数"

func SettingsVideoParallelValue(value any) string {
	return fmt.Sprint(value) + " 个"
}

const SettingsVisionModel = "识图模型"

const SettingsWatchGroup = "通知与自动刷新"

const SettingsWatchKeyword = "自动检查新回放"

const SetupAsrConfigured = "已有启用的识别渠道；还需音频提取组件，网络、配额和模型运行状态请在转写设置查看"

const SetupAsrMissing = "没有启用的识别渠道，请选择云端渠道或安装本地模型"

const SetupAsrTitle = "转写渠道"

const SetupConfigure = "配置"

const SetupInstallHelp = "安装说明"

const SetupInstallIntro = "这些组件安装在当前设备；其他设备需分别准备。安装完成后返回并重新检查。如果修改了系统 PATH，请完全退出并重开应用。"

const SetupIntro = "看在线视频无需先下载整段录像，也不需要转写组件。离线学习需下载；转写会复用本地录像，没有时获取临时音视频。以下检查互相独立，可按需要配置。"

const SetupLinuxInstall = "Linux：使用发行版的软件包管理器安装 FFmpeg 和 Poppler 工具。Ubuntu / Debian 可运行下面的命令。"

const SetupMacInstall = "macOS：已安装 Homebrew 时，在终端运行下面的命令。没有 Homebrew 时，先按 brew.sh 的官方说明安装。"

const SetupMediaTitle = "视频导入与音频提取"

const SetupModelConfigured = "已选择模型；实际可用性请在模型设置中测试"

const SetupModelMissing = "请选择模型并配置模型服务；运行环境就绪后才能读取配置"

const SetupModelTitle = "AI 模型与看图"

const SetupOtherInstall = "请在运行后台服务的设备上安装 FFmpeg 和 Poppler 工具，并确保它们在后台服务的 PATH 中。"

const SetupPdfHelperMissing = "内置 PDFKit 组件缺失，请更新或重新安装包含 PDF 组件的应用；安装 Poppler 不能替代此组件"

const SetupPdfTitle = "本机 PDF 导入"

const SetupPdfUnsupported = "当前平台的画布 PDF 导入尚未支持，不能仅靠安装组件启用；已导入的页面仍可查看"

const SetupRecheck = "重新检查"

const SetupTitle = "功能准备"

func SetupToolsMissing(tools string) string {
	return "缺少组件：" + tools
}

const SetupToolsPresent = "组件已找到（未测试解码）"

const SetupUnknown = "未检查成功，请重新检查"

const SetupVisionConfigured = "看图模型已配置；未发送图片进行连通性测试"

const SetupVisionMissing = "看图尚未配置：请选择支持图像的模型或单独配置视觉模型"

const SetupWindowsInstall = "Windows：从 FFmpeg 官网的 Windows 下载入口获取构建，确认包含 ffmpeg.exe 与 ffprobe.exe。将 FFmpeg 的 bin 目录加入用户 PATH 后重开应用。"

func ShellProfileTooltip(value any) string {
	return fmt.Sprint(value) + " · 个人资料"
}

const SlidesDownload = "下载 PPT"

const SlidesEmpty = "这节课没有 PPT 截图"

const SlidesEmptyBody = "平台只在老师投屏时生成截图"

const SlidesFollow = "跟随播放"

func SlidesIndex(idx any) string {
	return "第 " + fmt.Sprint(idx) + " 张"
}

const SlidesPageOnly = "仅换页"

func SlidesPosition(value any, length any, value2 any) string {
	return fmt.Sprint(value) + " / " + fmt.Sprint(length) + " · 从 " + fmt.Sprint(value2) + " 播放"
}

const SourceChallengeAnswer = "验证码"

const SourceExpired = "登录已过期"

func SourceExpiredBanner(name any) string {
	return "「" + fmt.Sprint(name) + "」的登录已过期，重新登录后继续同步"
}

func SourceFailed(error any) string {
	return "无法启动：" + fmt.Sprint(error)
}

const SourceFieldNotStored = "不会保存，下次登录需要重新填写"

func SourceFieldRequired(field any) string {
	return "请填写" + fmt.Sprint(field)
}

const SourceRestart = "重新启动"

const SourceSignInBody = "由插件完成登录，账号信息只保存在本机。"

const SourceSignInContinue = "继续登录"

const SourceSignInIncomplete = "无法完成登录：插件没有给出下一步。请重试。"

func SourceSignInTitle(name any) string {
	return "登录「" + fmt.Sprint(name) + "」"
}

const SourceSignOut = "退出登录"

func SourceSignedIn(name any) string {
	return "已登录「" + fmt.Sprint(name) + "」"
}

func SourceSignedInAs(account any) string {
	return "已登录为 " + fmt.Sprint(account)
}

const SourceSignedInPlain = "已登录"

const SourceSignedOut = "未登录"

func SourcesCannotInstall(error any) string {
	return "无法安装插件：" + fmt.Sprint(error)
}

func SourcesCannotRead(error any) string {
	return "无法读取插件：" + fmt.Sprint(error)
}

const SourcesChooseFolder = "选择插件文件夹"

const SourcesConsentBody = "插件会用你的账号访问上面列出的网站。只安装你信任的插件。"

const SourcesConsentHosts = "插件会访问的网站"

func SourcesConsentTitle(name any) string {
	return "安装「" + fmt.Sprint(name) + "」？"
}

const SourcesDisabled = "已停用"

const SourcesEmptyBody = "来源插件可以接入课程平台，同步课程、回放和作业。选择插件所在的文件夹即可安装。"

const SourcesEmptyTitle = "还没有来源插件"

const SourcesInstall = "从文件夹安装"

const SourcesInstallPlugin = "安装插件"

func SourcesInstalled(name any) string {
	return "已安装「" + fmt.Sprint(name) + "」"
}

const SourcesIntro = "每个插件提供一个或多个来源。登录信息只保存在本机。"

const SourcesMarketEmpty = "插件目录里还没有来源插件，也可以从文件夹安装。"

func SourcesMarketFailed(error any) string {
	return "无法读取插件目录：" + fmt.Sprint(error)
}

const SourcesMarketStale = "无法更新插件目录，以下是上次验证过的目录。"

const SourcesMarketTitle = "来源插件"

const SourcesMarketUnsupported = "不支持本机"

const SourcesMarketUnverified = "无法验证插件目录，以下是上次验证过的目录。"

const SourcesMarketUnverifiedEmpty = "无法验证插件目录，已忽略这次下载的目录。"

func SourcesMarketUpdate(version any) string {
	return "更新到 " + fmt.Sprint(version)
}

const SourcesMarketVerifying = "正在下载并校验…"

const SourcesMirrors = "下载线路…"

const SourcesMirrorsAdd = "添加线路"

const SourcesMirrorsAuto = "自动"

const SourcesMirrorsAutoBody = "按测速结果最快的优先，失败时依次换下一条"

const SourcesMirrorsBody = "插件目录和安装包从 GitHub 下载。直连不畅时可以经镜像线路下载；线路只转发文件，目录的签名和安装包的校验值照常核对，线路无法改动内容。"

const SourcesMirrorsChoice = "线路选择"

const SourcesMirrorsCustom = "自定义"

func SourcesMirrorsFailed(error any) string {
	return "不可用：" + fmt.Sprint(error)
}

const SourcesMirrorsName = "名称"

func SourcesMirrorsResult(latency any, speed any) string {
	return fmt.Sprint(latency) + " ms · " + fmt.Sprint(speed)
}

const SourcesMirrorsSwitch = "换线路"

const SourcesMirrorsTemplate = "地址"

func SourcesMirrorsTemplateHelper(example any) string {
	return "以 https 开头，GitHub 原地址处写 " + fmt.Sprint(example)
}

const SourcesMirrorsTest = "测速"

func SourcesMirrorsTestedAt(time any) string {
	return "上次测速 " + fmt.Sprint(time)
}

const SourcesMirrorsTesting = "测速中…"

const SourcesMirrorsTitle = "下载线路"

const SourcesMirrorsUntested = "未测速"

func SourcesMirrorsUsing(name any) string {
	return "正在使用 " + fmt.Sprint(name)
}

const SourcesNone = "未安装来源插件"

const SourcesRemoveBody = "插件和它保存的登录信息会被删除。已同步的课程和课次留在本机，但不能再刷新。"

const SourcesRemovePlugin = "卸载插件"

func SourcesRemoveTitle(name any) string {
	return "卸载「" + fmt.Sprint(name) + "」？"
}

func SourcesSummary(count any, signed any) string {
	return fmt.Sprint(count) + " 个来源 · " + fmt.Sprint(signed) + " 个已登录"
}

const SourcesTitle = "来源"

const SourcesUnavailable = "这个插件不支持本机的系统，无法安装。"

func SourcesVersion(version any) string {
	return "版本 " + fmt.Sprint(version)
}

const StateLoadFailed = "无法加载"

const StateSessionExpired = "登录已失效"

const StorageAssistant = "助手对话、记忆与插件"

const StorageCache = "转写缓存与未完成下载"

func StorageCacheCleanable(value any) string {
	return "可释放 " + fmt.Sprint(value) + " · 进行中和暂停的任务会保留"
}

const StorageCacheEmpty = "没有可清理的缓存"

func StorageCleanFailed(e any) string {
	return "清理失败：" + fmt.Sprint(e)
}

const StorageCleanSyncBody = "其中是同步时被替换或在其他设备上删除的文件副本，清空后无法找回。"

const StorageCleanSyncTitle = "清空同步冲突与回收站？"

const StorageCleaningCache = "正在清理缓存"

const StorageCompact = "整理索引"

const StorageCompactAction = "整理"

const StorageCompactHint = "合并写入日志、回收空闲空间，让搜索和列表更快"

const StorageCompacting = "正在整理索引"

const StorageEmptying = "正在清空"

func StorageFreed(value any) string {
	return "已释放 " + fmt.Sprint(value)
}

const StorageModels = "本地语音模型"

const StorageNoFiles = "没有文件"

const StorageRuntime = "助手运行环境（Node.js 与依赖）"

const StorageSync = "同步冲突与回收站"

func StorageSyncSize(value any) string {
	return fmt.Sprint(value) + " · 同步时替换或删除的文件副本"
}

const StorageTitle = "存储空间"

const StreamBlur = "模糊显现"

const StreamFade = "淡入"

const StreamScreen = "屏幕"

const StreamStudent = "学生"

const StreamTeacher = "教师"

const StreamTypewriter = "打字机"

const StreamWave = "波浪"

const StudyAddLayer = "添加图层"

const StudyAddText = "添加文字笔记"

const StudyAgentCheckingQuotes = "正在核对引文…"

func StudyAgentListening(label any) string {
	return "正在听「" + fmt.Sprint(label) + "」"
}

func StudyAgentLooking(label any) string {
	return "正在看「" + fmt.Sprint(label) + "」"
}

const StudyAgentLookingAround = "正在查看画布…"

const StudyAgentLookingShot = "正在看截图…"

const StudyAgentPreparing = "正在准备资料…"

func StudyAgentReading(label any) string {
	return "正在读「" + fmt.Sprint(label) + "」"
}

const StudyAgentReadingCourse = "正在读课程资料…"

const StudyAgentSearchingCourse = "正在检索本课程资料…"

const StudyAgentThinking = "正在组织回答…"

func StudyAgentTranscribing(label any) string {
	return "正在转写「" + fmt.Sprint(label) + "」的片段"
}

const StudyAllowImages = "看图"

const StudyAllowImagesHint = "允许读取圈选的画面"

func StudyAnswerLabel(letter any) string {
	return "答案：" + fmt.Sprint(letter)
}

const StudyAskAI = "问 AI"

const StudyAskHint = "助手会收到所选卡片的文字和视频时间点"

const StudyAskSubmit = "提问"

const StudyAskTitle = "向课程助手提问"

const StudyBack = "返回课程"

const StudyBindMore = "添加卡片"

const StudyBoundCard = "未命名卡片"

func StudyBoundInk(count int) string {
	return fmt.Sprint(count) + " 笔手写"
}

const StudyBoundLecture = "课程视频"

const StudyBoundNeedsVision = "没有可读文字，需要「看图」"

const StudyBoundReadable = "含可读文字"

const StudyBoundRegion = "局部"

const StudyBoundRemove = "移除此资料"

const StudyCaptureCard = "截图成卡片"

const StudyCaptureLasso = "截图套索"

const StudyCaptureOutside = "请把整个选区移入屏幕后再截图"

const StudyCardAsk = "加入对话，或拖进输入框"

const StudyCardCollapse = "折叠模块"

const StudyCardDelete = "删除模块"

const StudyCardEditKnowledge = "编辑知识卡片"

const StudyCardExpand = "展开模块"

const StudyCardFoldColumn = "收起整列"

const StudyCardLimit = "画布卡片超过 2000 张"

const StudyCardMore = "更多"

func StudyCardPdfPage(title any, page any, count any) string {
	return fmt.Sprint(title) + " · 第 " + fmt.Sprint(page) + "/" + fmt.Sprint(count) + " 页"
}

const StudyCardSelectText = "选择文档文字"

const StudyCitationUnavailable = "来源已变更或不可用，保留当前阅读位置"

const StudyClassic = "使用传统播放器"

const StudyComposerDrop = "松开，加入这段对话"

const StudyComposerHint = "就画布上的资料提问"

const StudyConversationEmpty = "围绕指定资料持续提问，回答和整理结果都保留在此对话中。"

const StudyConversationFull = "这段对话已满 300 轮，请开始新对话"

func StudyConversationSaveFailed(error any) string {
	return "无法保存对话：" + fmt.Sprint(error)
}

const StudyConversationUnsaved = "无法整理资料：画布还没保存，请稍后重试"

const StudyCourseScopeChip = "全课程"

const StudyCourseScopeHint = "允许助手检索并读取本课程的本地资料；关闭时只读你绑定的内容"

const StudyCourseware = "放入课程资料"

func StudyCoverageLead(parts string) string {
	return "依据 " + parts
}

const StudyCoverageNone = "未引用资料原文，内容可能来自常识或推断"

func StudyCoveragePassages(count int) string {
	return fmt.Sprint(count) + " 段原文"
}

func StudyCoverageStale(count int) string {
	return fmt.Sprint(count) + " 处已失效"
}

func StudyCoverageTranscript(count int) string {
	return fmt.Sprint(count) + " 句讲解"
}

func StudyCoverageVisual(count int) string {
	return fmt.Sprint(count) + " 处看图"
}

const StudyDefaultLayer = "学习笔记"

const StudyEditShape = "编辑形状锚点"

const StudyEditText = "编辑文字"

const StudyEraser = "橡皮 E"

const StudyEvidenceInk = "手写"

const StudyEvidenceLocate = "定位"

const StudyEvidenceMissing = "无法定位：来源已移除或所在图层已隐藏"

const StudyEvidenceOtherLecture = "此来源属于其他课次，请先打开对应课次"

const StudyEvidenceSeen = "看过的图像"

const StudyEvidenceSeenBody = "助手读过这块选区的图像，画面内容以原图为准"

func StudyEvidenceSemantics(number int, locator string) string {
	return "引用 " + fmt.Sprint(number) + "：" + locator
}

const StudyEvidenceStale = "来源已失效：原文已修改或删除"

const StudyExcerpt = "摘录"

const StudyExcerptHint = "把选区截成带来源的卡片"

const StudyExitCanvas = "退出画布"

func StudyExportError(error string) string {
	return "无法导出：" + error
}

const StudyExportRecovery = "导出恢复文件"

const StudyFileTooLarge = "文件超过 300 MiB，请拆分后导入"

const StudyFit = "显示全部材料"

const StudyHandTool = "移动 H"

const StudyHighlighter = "荧光笔"

const StudyIdeaExplain = "把选中的部分讲得更易懂"

const StudyIdeaQuiz = "出三道题考考我"

const StudyIdeaSummary = "概括这一页的要点"

const StudyImageFailed = "无法读取图片，请重新导入"

const StudyImport = "导入"

func StudyImportError(error string) string {
	return "无法导入素材：" + error
}

const StudyImporting = "正在导入并处理素材…"

const StudyInkLimit = "本画布笔迹已接近上限，请另建课次画布"

const StudyInspectSources = "查看本次资料内容"

const StudyKeepEditing = "继续编辑"

func StudyLayerName(number int) string {
	return "图层 " + fmt.Sprint(number)
}

const StudyLayerUnavailable = "当前图层已隐藏或锁定"

const StudyLayers = "图层"

const StudyLegacyOperations = "历史画布操作已停用，回答仍保留在对话中。"

const StudyLinkCancel = "取消待连接起点"

const StudyLinkDescription = "点击下方来源定位到关联选区"

const StudyLinkFinish = "连接到此选区"

const StudyLinkPending = "已记住起点，请圈选另一处，再选“连接到此选区”"

const StudyLinkStart = "设为链接起点"

const StudyLinkTitle = "选区链接"

const StudyLocalVideoError = "无法播放此视频。请检查本地文件后重新打开画布。"

const StudyLocked = "图层已锁定或隐藏"

const StudyMaterials = "学习材料"

const StudyMoveHint = "拖动卡片标题移动 · 右下角缩放 · ⌘/Ctrl + 滚轮缩放"

const StudyNewText = "添加文字"

const StudyOpenCanvas = "打开画布"

func StudyOpenError(error string) string {
	return "无法打开学习画布：" + error
}

const StudyOpening = "正在打开画布"

const StudyPenTool = "钢笔 D"

func StudyPenWidth(width string) string {
	return "笔宽 " + width
}

const StudyPendingSave = "等待保存"

const StudyPickLayer = "请先选择可编辑的图层"

const StudyPickSources = "选择参考资料"

const StudyPlayer = "播放器"

const StudyPluginBatchApply = "应用改动"

func StudyPluginBatchCreate(value any) string {
	return "新增「" + fmt.Sprint(value) + "」"
}

func StudyPluginBatchDelete(value any) string {
	return "删除「" + fmt.Sprint(value) + "」"
}

func StudyPluginBatchFailed(value any) string {
	return "无法修改画布：" + fmt.Sprint(value)
}

func StudyPluginBatchTitle(value any) string {
	return "允许「" + fmt.Sprint(value) + "」修改画布吗？"
}

const StudyPluginBatchUndo = "应用后可以撤销。"

func StudyPluginBatchUpdate(value any) string {
	return "覆盖「" + fmt.Sprint(value) + "」"
}

const StudyPluginObjectExport = "导出对象"

func StudyPluginObjectExported(value any) string {
	return "已导出「" + fmt.Sprint(value) + "」"
}

const StudyPluginObjectRemove = "移除对象"

const StudyPluginObjects = "画布上的插件对象"

const StudyPlugins = "插件"

const StudyPluginsNoTools = "已启用的插件没有画布工具"

const StudyQuickHint = "问问选中的内容…"

const StudyQuickKeys = "Enter 发送 · Esc 返回"

const StudyQuickNeedsVision = "选中的内容没有可读文字。打开「看图」后，助手才能读懂它。"

func StudyQuickSendFailed(error string) string {
	return "无法发送选区问题：" + error
}

const StudyRecognizeConsent = "将选中的笔迹图片发送给已配置的模型，可能产生费用。识别后可以修改文字，再确认插入；原笔迹保留。"

const StudyRecognizeFailed = "识别未完成，请检查模型配置后重试"

const StudyRecognizeInk = "手写转文字"

const StudyRecognizeInsert = "插入文字卡片"

const StudyRecognizePrompt = "请调用 inspect_canvas_region 查看 selected-ink 中的手写内容，并只输出逐字转录的文字，保留换行。不要解答、解释、添加标题或引用；不能辨认的字写作[无法辨认]。"

const StudyRecognizeReview = "校正识别结果"

const StudyRecognizeStart = "开始识别"

const StudyRecognizeWorking = "正在识别手写…"

const StudyRecoverySaved = "已导出恢复文件；素材仍在本机课程目录"

const StudyRedo = "重做"

const StudyReferencesRemoved = "已移除失效或已隐藏的资料引用。请确认参考资料后再发送；如需手写截图，请重新框选。"

const StudyRelate = "关联"

const StudyRelateHere = "连到这里"

const StudyRelateHereHint = "从关联起点连到这处选区"

const StudyRelateHint = "设为关联起点，再圈选另一处连接"

const StudyRemoveCards = "移除所选卡片（素材文件保留）"

const StudyRemoveSource = "移出对话"

const StudyReturn = "返回原处"

const StudyReturnKeys = "Esc 或 Alt+←"

const StudyReturnKeysMac = "Esc 或 ⌘["

const StudyRotateSelection = "顺时针旋转 90°"

const StudyRotateUnsupported = "旋转支持笔迹和展开的图片；请取消选择其他类型或锁定的对象"

const StudySaveFailed = "保存失败 · 点击重试"

const StudySaved = "已保存到本机"

const StudySaving = "保存中…"

const StudySelectDocument = "图片与 PDF"

const StudySelectInk = "手写笔迹"

const StudySelectOther = "其他卡片"

const StudySelectText = "文字与知识卡片"

const StudySelectTool = "选择 V"

const StudySelectVideo = "视频"

const StudySelectionChanged = "操作期间画布已改变，请重新选择后重试"

const StudySelectionClear = "取消选择"

const StudySelectionClipped = "选区有一部分在屏幕外，请缩小画布让选区完整显示后重试"

const StudySelectionEmpty = "请先圈选资料或笔迹"

func StudySelectionError(error string) string {
	return "选区操作失败：" + error
}

const StudySelectionFilters = "套索选择对象"

const StudySelectionImageLimit = "每次最多附两张选区图，请缩小选区或关闭“附选区图”"

const StudySelectionMore = "选区工具"

const StudySelectionNeedsSource = "请圈选卡片、资料内容或笔迹后再附图提问"

func StudySelectionSource(label string) string {
	return "定位来源 · " + label
}

const StudySend = "发送"

const StudyShapeDragHint = "拖动端点或顶点，虚线辅助对齐；确认后保存"

const StudyShapeUnavailable = "请选择一笔完整的直线、箭头、圆或多边形"

const StudyShelf = "课次素材"

const StudyShelfBack = "返回全部资料"

func StudyShelfChapterLines(count int) string {
	return fmt.Sprint(count) + " 句字幕"
}

const StudyShelfFiles = "资料"

func StudyShelfFilesCount(count int) string {
	return fmt.Sprint(count) + " 个文件"
}

const StudyShelfFromVideo = "字幕 · 资料 · 练习"

const StudyShelfHint = "拖到画布上，或点 ＋ 放到视野中央"

const StudyShelfHintTouch = "长按拖到画布上，或点 ＋ 放到视野中央"

func StudyShelfLoadFailed(error any) string {
	return "无法加载：" + fmt.Sprint(error)
}

const StudyShelfNoCourse = "课程信息还在加载，请稍后再打开"

const StudyShelfNoFiles = "这门课还没有本机资料。可以先在资料库下载，或用「插入图片、PDF 或文件」导入。"

const StudyShelfNoNotes = "这节课还没有笔记"

const StudyShelfNoPractice = "还没有练习。在播放器的「笔记」里可以让助手出一套题。"

const StudyShelfNoTranscript = "这节课还没有字幕"

const StudyShelfNotes = "笔记"

const StudyShelfNow = "回到正在播放"

const StudyShelfPanel = "打开播放器侧栏"

const StudyShelfPlace = "放到画布"

const StudyShelfPractice = "练习"

const StudyShelfThisLecture = "本课次"

const StudyShelfThreadHint = "放到画布上，在下方输入框提问"

const StudyShelfThreads = "对话"

const StudyShelfTranscript = "字幕"

const StudyShelfUnsupported = "暂不支持此格式"

const StudyShot = "选区截图"

const StudySidebar = "课程资料与助手"

const StudySnapshotDrag = "拖入对话，附上选区截图"

const StudySnapshotRemove = "移除截图"

const StudySnapshotRetry = "重试上传"

const StudySnapshotTooLarge = "截图超过 3 MiB，请缩小选区"

const StudySnapshotVideoFailed = "无法取得选区中的视频画面，请检查本地视频和 ffmpeg 后重试"

func StudySourcePage(page any) string {
	return "第 " + fmt.Sprint(page) + " 页"
}

const StudySourceShow = "查看来源"

func StudySourcesCount(count int) string {
	return fmt.Sprint(count) + " 份资料"
}

const StudySourcesHide = "收起资料列表"

func StudySourcesReading(label string) string {
	return "正在读 · " + label
}

const StudySourcesShow = "展开资料列表"

const StudyStageBody = "选中卡片、圈出一段笔迹，或把卡片拖进输入框，回答会以这些资料为依据。"

const StudyStageCollapse = "收起对话"

func StudyStageExpand(count int) string {
	return "展开对话（" + fmt.Sprint(count) + " 轮）"
}

const StudyStageTitle = "从画布上的资料问起"

const StudyStop = "停止"

const StudySuggestFollowupHint = "填入输入框，确认后再发送"

func StudySuggestOpen(locator string) string {
	return "回看 " + locator
}

const StudyTextCard = "文字笔记"

const StudyTextHint = "写下思考，或粘贴 Markdown…"

const StudyThreadActive = "正在对话"

const StudyThreadActiveHint = "这段对话在画布下方的输入框上继续。"

const StudyThreadEmpty = "还没有提问"

const StudyThreadNew = "新对话"

const StudyThreadOpen = "继续"

func StudyThreadTitle(n any) string {
	return "对话 " + fmt.Sprint(n)
}

func StudyThreadTurns(count int) string {
	return func() string {
		switch {
		case count == 0:
			return "还没有提问"
		}
		return fmt.Sprint(count) + " 轮问答"
	}()
}

const StudyTitle = "学习画布"

const StudyToggleLocked = "锁定或解锁"

const StudyToggleVisible = "显示或隐藏"

func StudyTranscriptExcerpt(count int) string {
	return "（只放入了前 " + fmt.Sprint(count) + " 句）"
}

func StudyTranscriptTitle(range_ any) string {
	return "字幕 · " + fmt.Sprint(range_)
}

const StudyTurnInterrupted = "这次回答中断了"

const StudyTurnReopened = "应用重开时这个问题还没答完，可以重试"

const StudyTurnRetry = "重试"

const StudyUnknownVersion = "画布版本不受支持"

const StudyUnsaved = "画布尚未保存"

const StudyUnsavedBody = "请检查本机服务后重试，或先导出恢复文件。当前内容仍保留在本窗口。"

func StudyVideoRange(from any, to any) string {
	return fmt.Sprint(from) + "–" + fmt.Sprint(to) + " 秒"
}

const StudyVisualChanged = "圈选的画面已经变了，请重新圈选后再加入对话"

const StudyVisualDimensionsTooLarge = "原图尺寸过大，请缩小图片后重试"

const StudyVisualExpired = "选区图像已失效，请重新圈选并附图；问题已保留"

const StudyVisualFileTooLarge = "选区原图超过 8 MiB，请缩小图片后重试"

const StudyVisualPayloadTooLarge = "选区图像过大，请缩小选区后重试"

const StudyVisualReadFailed = "选区原图读取失败，请确认资料已下载后重试"

const StudyVisualRegionInvalid = "选区范围无效，请重新选择；问题已保留"

const StudyWriteHint = "触控笔 / 鼠标书写 · 手指移动画布"

const StudyZoomIn = "放大"

const StudyZoomOut = "缩小"

const SuggestBiggerText = "把界面字号调大一点"

const SuggestConceptElsewhere = "刚才提到的概念在哪些课里出现过？"

const SuggestCourseRecent = "这门课最近几节讲了什么？"

const SuggestDownloadRecent = "下载这门课最近三节的视频和转写"

const SuggestPauseDownloads = "暂停所有下载"

const SuggestQuizCourse = "出一组题考考我这门课的重点"

const SuggestRecentUnwatched = "最近有哪些课还没看？"

const SuggestRemember = "记住：我更喜欢先给结论的简短回答"

const SuggestReviewNotes = "把这节课整理成一份复习笔记"

const SuggestSearchMidterm = "在我的课程库里搜索「期中」"

const SuggestSpeed = "把播放速度调到 1.5 倍"

const SuggestSummarizeLecture = "总结这节课到目前为止讲了什么"

const SuggestUnwatched = "帮我挑出还没看的课"

const SuggestWeeklyReview = "每周日晚上帮我复习本周的课"

const SuggestWhatCanYouDo = "这个 App 能帮我做什么？"

func SyncCreated(value any) string {
	return "新建 " + fmt.Sprint(value)
}

func SyncDeleted(value any) string {
	return "删除 " + fmt.Sprint(value)
}

const SyncInSync = "两边已经一致"

func SyncTrashed(value any) string {
	return "移到废纸篓 " + fmt.Sprint(value)
}

func SyncUnchanged(value any) string {
	return "未变 " + fmt.Sprint(value)
}

func SyncUpdated(value any) string {
	return "更新 " + fmt.Sprint(value)
}

func SyncUploaded(value any) string {
	return "上传 " + fmt.Sprint(value)
}

const TaskTemplateDeadlinesName = "每天早上汇总近期截止"

const TaskTemplateDeadlinesPrompt = "列出今天起 7 天内的作业、考试和截止事项，按日期排序，提醒最紧急的两件；如果有已经过期但没完成的也指出来。"

const TaskTemplateQuizName = "每周一出一组复习题"

const TaskTemplateQuizPrompt = "回顾上周转写过的课程，为每门课出一组 10 道中等难度的选择题并保存为练习，重点覆盖上周的新内容。"

const TaskTemplateReviewName = "每晚复盘今天的课"

const TaskTemplateReviewPrompt = "看看今天有哪些课的转写已经完成，为每节课写一份简明的复盘笔记并保存：核心概念、关键公式与例题、老师强调的要点，附时间戳。没有新课就简单说明。"

const TasksAllowHint = "运行时只允许勾选的写入；都不选则只读，结果写在总结里。任务读到的转写稿、课件等资料不能让它做别的修改。"

const TasksAllowLabel = "允许它"

func TasksAllowed(value any) string {
	return "允许：" + fmt.Sprint(value)
}

const TasksBusy = "助手正在回复，请稍后再查看"

const TasksCatchUp = "错过时补跑"

const TasksCatchUpHint = "应用在计划时间未运行时，12 小时内启动会补跑一次"

const TasksCostLimit = "单次上限"

const TasksCostLimitCatchUp = "单次上限 · 补跑"

const TasksCostLimitHelper = "超过后中止这次运行（按模型价格估算）"

const TasksCostLimitLabel = "单次费用上限"

func TasksCount(length any) string {
	return fmt.Sprint(length) + " 个任务"
}

const TasksDeleteBody = "以后不会再运行。已经生成的对话、笔记和练习会保留，运行记录仍可在「运行记录」里查看。"

const TasksDeleteTitle = "删除任务？"

const TasksDeletedSuffix = "（已删除）"

const TasksDeletedTask = "已删除的任务"

const TasksEditTitle = "编辑定时任务"

func TasksEnabledCount(length any) string {
	return fmt.Sprint(length) + " 个启用"
}

const TasksFromBlank = "从空白开始"

const TasksIntroBody = "每次运行都会开一个新对话，结果可在「运行记录」里查看。任务只在 Coursive 打开时执行；错过的时间可以在 12 小时内启动时补跑。"

const TasksIntroTitle = "让助手按计划替你做事"

const TasksLastResult = "上次结果"

const TasksLimits = "限制"

const TasksManualSuffix = " · 手动"

const TasksNeverRun = "还没运行"

const TasksNew = "新建任务"

const TasksNewTitle = "新建定时任务"

func TasksNextRun(value any) string {
	return "下次 " + fmt.Sprint(value)
}

const TasksNextRunLabel = "下次运行"

func TasksNextUp(value any, value2 any) string {
	return "下一个「" + fmt.Sprint(value) + "」" + fmt.Sprint(value2)
}

const TasksNoNextRun = "没有下次运行"

const TasksNoRunsYet = "还没有运行过。可以点「立即运行」先试一次。"

const TasksNotFound = "任务不存在"

const TasksPrompt = "任务内容"

const TasksPromptLabel = "让助手做什么"

const TasksReadOnly = "只读：结果写在总结里，不改动课程库"

func TasksRecentRuns(length any) string {
	return "最近 " + fmt.Sprint(length) + " 次"
}

func TasksRunNotice(value any, label any, why any) string {
	return "定时任务「" + fmt.Sprint(value) + "」" + fmt.Sprint(label) + fmt.Sprint(why)
}

const TasksRunNow = "立即运行"

func TasksRunReason(value any) string {
	return "：" + fmt.Sprint(value)
}

const TasksRunningNow = "正在运行"

const TasksRuns = "运行记录"

const TasksRunsEmpty = "任务运行后，结果会按时间出现在这里"

const TasksRunsNoMatch = "没有符合条件的运行"

const TasksStarted = "已开始运行"

const TasksSubtitle = "按计划在新对话里复盘、出题、汇总截止事项"

const TasksWorkdays = "工作日"

const TasksWriteNotes = "保存笔记与大纲"

const TasksWriteQuizzes = "创建练习"

const TasksWriteSchedule = "标记与修改日程"

func TermAutumn(y any) string {
	return fmt.Sprint(y) + " 秋"
}

func TermSpring(value any) string {
	return fmt.Sprint(value) + " 春"
}

const ThemeDark = "深色"

const ThemeLight = "浅色"

const ThemeSystem = "跟随系统"

const ThinkingHintHigh = "推理更充分，等待更久"

const ThinkingHintLow = "简单推理，适合日常问答"

const ThinkingHintMax = "尽可能深入，最慢，花费也最多"

const ThinkingHintMedium = "兼顾速度与深度"

const ThinkingHintMinimal = "只做最少的推理，回复最快"

const ThinkingHintOff = "直接回答，不做额外推理"

const ThinkingLow = "低"

const ThinkingMax = "最高"

const ThinkingMinimal = "最少"

const ThinkingSheetBody = "更强的思考回答更周全，也更慢、更费 token。"

const ThinkingTitle = "思考强度"

func TimeDaysAgo(inDays any) string {
	return fmt.Sprint(inDays) + " 天前"
}

const TimeExpired = "已过期"

func TimeHoursAgo(inHours any) string {
	return fmt.Sprint(inHours) + " 小时前"
}

const TimeJustNow = "刚刚"

func TimeMinutesAgo(inMinutes any) string {
	return fmt.Sprint(inMinutes) + " 分钟前"
}

func TimeToday(hm any) string {
	return "今天 " + fmt.Sprint(hm)
}

func TimeTomorrow(hm any) string {
	return "明天 " + fmt.Sprint(hm)
}

func TimeYesterday(hm any) string {
	return "昨天 " + fmt.Sprint(hm)
}

const TimestampOpenLecture = "打开对应课次并跳到这里"

const TimestampSeek = "跳到这里播放"

const ToolArgs = "参数"

const ToolChangeSettings = "调整设置"

const ToolControlPlayer = "控制播放"

const ToolCreateQuiz = "生成练习"

const ToolCreateTask = "创建定时任务"

const ToolCurrentContext = "读取当前界面"

const ToolDownload = "下载"

const ToolEditMemory = "改记忆"

const ToolEditSchedule = "修改日程"

const ToolFindTools = "加载工具"

const ToolForget = "忘掉"

const ToolHelp = "查阅功能说明"

const ToolLectureDetail = "查看课次详情"

const ToolListCourses = "查看课程"

const ToolListCourseware = "查看课件"

const ToolListDownloads = "查看下载"

const ToolListLectures = "查看课次"

const ToolListMemory = "翻看记忆"

const ToolListNotes = "查看笔记"

const ToolListQuizzes = "查看练习"

const ToolListSchedule = "查看日程"

const ToolListSlides = "查看 PPT"

const ToolManageDownloads = "管理下载"

const ToolMarkSchedule = "标记日程"

const ToolOpenPage = "打开页面"

const ToolOrganizeCourses = "整理课程"

const ToolPrepare = "准备操作"

const ToolPrepareDelete = "准备删除文件"

const ToolPrepareDownload = "准备下载"

const ToolPreparePlugin = "准备开关插件"

const ToolPrepareTask = "准备定时任务"

const ToolReadAttempts = "查看作答"

const ToolReadCourseware = "阅读课件"

const ToolReadNote = "阅读笔记"

const ToolReadOutline = "读取大纲"

const ToolReadSettings = "读取设置"

const ToolReadTranscript = "阅读文字稿"

const ToolRecall = "回想"

const ToolRemember = "记下来"

const ToolResult = "结果"

const ToolReturned = "已返回"

const ToolSaveNote = "保存笔记"

const ToolSaveOutline = "保存大纲"

const ToolSearchCourseware = "搜索课件"

const ToolSearchLibrary = "搜索课程库"

const ToolSyncCourses = "同步课程"

const ToolTogglePlugin = "开关插件"

const ToolViewImage = "查看插图"

const TranscriptCopied = "已复制全文"

const TranscriptCopyAll = "复制全文"

const TranscriptEmpty = "还没有文字稿"

const TranscriptEmptyBody = "转写会提取音频并生成带时间戳的文字，之后可以搜索、做字幕，也能让助手整理笔记。"

const TranscriptFindHint = "在文字稿中查找"

const TranscriptFollowOff = "跟随播放：关"

const TranscriptFollowOn = "跟随播放：开"

func TranscriptMatches(length any) string {
	return fmt.Sprint(length) + " 处"
}

const UsageBreakdown = "明细"

const UsageCacheHit = "缓存命中"

const UsageCacheRate = "缓存命中率"

func UsageCacheTokens(value any) string {
	return "命中 " + fmt.Sprint(value) + " tokens"
}

func UsageCachedSuffix(value any) string {
	return "（缓存 " + fmt.Sprint(value) + "）"
}

const UsageCallCount = "调用次数"

func UsageCalls(value any) string {
	return fmt.Sprint(value) + " 次"
}

func UsageCallsInRange(label any) string {
	return fmt.Sprint(label) + "内的调用"
}

const UsageChart = "图表"

func UsageChartSemantics(value any) string {
	return "每日" + fmt.Sprint(value) + "柱状图，可切换为表格查看"
}

func UsageCompare(span any, value any, value2 any) string {
	return "比" + fmt.Sprint(span) + fmt.Sprint(value) + " " + fmt.Sprint(value2) + "%"
}

const UsageCost = "费用"

const UsageDaily = "每日用量"

const UsageDeletedChat = "已删除的对话"

const UsageEmpty = "这段时间没有用量"

const UsageEmptyBody = "和助手对话、生成大纲或识图后，这里会按天统计 token 和估算费用。"

const UsageEstimatedCost = "估算费用"

func UsageFailedCalls(value any) string {
	return "失败 " + fmt.Sprint(value) + " 次"
}

func UsageFirstToken(value any) string {
	return "首字 " + fmt.Sprint(value) + "s"
}

const UsageInput = "输入"

func UsageInputOutput(value any, value2 any) string {
	return "输入 " + fmt.Sprint(value) + " · 输出 " + fmt.Sprint(value2)
}

func UsageInputTokens(value any) string {
	return "输入 " + fmt.Sprint(value)
}

const UsageLastYear = "去年同期"

const UsageNoCalls = "没有调用记录"

func UsageNoneIn(span any) string {
	return fmt.Sprint(span) + "无用量"
}

const UsageOutput = "输出"

func UsagePreviousDays(days any) string {
	return "前 " + fmt.Sprint(days) + " 天"
}

const UsagePurpose = "用途"

const UsagePurposeChat = "对话"

const UsagePurposeMemory = "记忆沉淀"

const UsagePurposeTest = "连接测试"

const UsageRecentCalls = "最近调用"

func UsageSameAs(span any) string {
	return "与" + fmt.Sprint(span) + "持平"
}

const UsageTable = "表格"

const UsageTokens = "token 用量"

const UsageView = "查看用量"

const VisionBody = "只用来把 PPT 和课件插图转写成文字，交给不能看图的对话模型。结果按图片缓存，同一张图只识别一次。"

func VisionNative(value any) string {
	return "当前模型可以直接看 PPT 和插图" + fmt.Sprint(value)
}

func VisionNativeFallback(vision any) string {
	return "；换成纯文本模型时使用 " + fmt.Sprint(vision)
}

const VisionNeeded = "当前模型不能看图。选一个识图模型，助手写笔记时才能核对 PPT 和课件插图"

const VisionNoModels = "已配置的服务商里没有支持图片输入的模型。"

const VisionNone = "不使用"

func VisionUnavailable(vision any) string {
	return fmt.Sprint(vision) + " 不可用（检查服务商密钥）"
}

func VisionUsing(vision any) string {
	return fmt.Sprint(vision) + " · 为纯文本模型转写 PPT 和插图"
}

func WatchAwaiting(awaiting any) string {
	return "正在等待 " + fmt.Sprint(awaiting) + " 节课的回放"
}

const WatchBody = "登录来源后自动检查：刚下课的课每 10 分钟查一次回放，之后逐渐放缓；其他课程每 6 小时同步一次；学习平台每 30 分钟同步作业和通知。有变化时立即提醒，课程页也会自动刷新。"

const WatchCheckNow = "立即检查"

const WatchCheckStarted = "已开始检查，有变化会马上通知你"

func WatchLastError(err any) string {
	return "最近一次检查失败：" + fmt.Sprint(err)
}

func WatchNextCheck(value any) string {
	return "下次检查 " + fmt.Sprint(value)
}

const WatchNoneAwaiting = "没有等待中的回放"

const WatchTitle = "后台自动检查新回放和新课次"

const WebdavAccount = "账号"

const WebdavAlistHint = "地址形如 http://主机:5244/dav/，账号和密码用 AList 的用户"

const WebdavAppPassword = "应用密码"

const WebdavAutoHint = "打开应用时、每 30 分钟，以及笔记改动约 2 分钟后"

const WebdavBody = "在电脑、手机之间同步课程库和助手对话：笔记、转写稿、PPT 截图、题目与学习记录、播放进度和书签。每台设备只上传改动过的文件；两边都改过的文件保留较新的一份，另一份放进冲突文件夹。"

func WebdavConflicts(value any) string {
	return fmt.Sprint(value) + " 个文件在两台设备上都改过，保留了较新的版本，另一份在冲突文件夹里"
}

const WebdavConnect = "连接网盘"

const WebdavConnectAndSync = "连接并同步"

func WebdavConnectedTo(host any) string {
	return "已连接 " + fmt.Sprint(host)
}

const WebdavEditAccount = "修改账号"

func WebdavErrors(length any, first any) string {
	return fmt.Sprint(length) + " 个文件没有同步成功：" + fmt.Sprint(first)
}

const WebdavFolder = "同步目录"

const WebdavFolderHelper = "在网盘里新建，所有设备填同一个"

const WebdavJianguoyun = "坚果云"

const WebdavJianguoyunHint = "账号是坚果云登录邮箱；在网页版「账户信息 → 安全选项 → 第三方应用管理」添加应用，得到应用密码"

const WebdavJianguoyunUser = "坚果云账号（邮箱）"

const WebdavKeepPassword = "留空则沿用已保存的密码"

const WebdavMedia = "同步视频与录音"

const WebdavMediaHint = "文件很大，会占用较多网盘空间和流量；不同步时可以在其他设备上重新下载"

const WebdavMediaOff = "不同步，在其他设备上重新下载"

const WebdavMediaOn = "一起同步"

const WebdavMediaTitle = "视频与录音"

const WebdavNeverSynced = "还没同步过"

const WebdavNextcloudHint = "地址形如 https://域名/remote.php/dav/files/用户名/；在「个人设置 → 安全」创建应用密码"

const WebdavOtherHint = "填写服务商提供的 WebDAV 地址、账号和密码"

const WebdavPasswordSealed = "用本机密钥加密保存"

const WebdavPrivacy = "登录状态、模型密钥和本机设置不会上传，每台设备分别登录。同步目录里的文件按原样保存，不做加密，请使用可信的网盘。"

const WebdavServer = "服务器地址"

const WebdavStop = "停止同步"

const WebdavStopBody = "会删除本机保存的 WebDAV 账号和同步记录。网盘和本机上的文件都会保留；之后重新设置时会按文件内容重新比对。"

const WebdavStopTitle = "停止同步？"

const WebdavSubtitle = "用坚果云、Nextcloud 等网盘在设备间同步"

func WebdavSyncedAgo(value any) string {
	return fmt.Sprint(value) + "同步"
}

const WebdavSyncing = "正在同步"

const WebdavTitle = "WebDAV 同步"

const WeekdayFri = "周五"

const WeekdayMon = "周一"

func WeekdayName(value any) string {
	return "周" + fmt.Sprint(value)
}

const WeekdayShort1 = "一"

const WeekdayShort2 = "二"

const WeekdayShort3 = "三"

const WeekdayShort4 = "四"

const WeekdayShort5 = "五"

const WeekdayShort6 = "六"

const WeekdayShort7 = "日"

const WeekdayWed = "周三"

func WorkspaceAptExit(code any) string {
	return "apt-get 退出码 " + fmt.Sprint(code)
}

const WorkspaceBody = "Coursive 在手机上运行一个精简的 Ubuntu 环境，下载、转写、同步和助手都在里面工作，和电脑版一样。首次需要下载约 30 MB，安装后占用约 140 MB，只保存在本应用内。"

const WorkspaceChecksumFailed = "下载的文件校验失败，请重试"

const WorkspaceConnecting = "正在连接镜像…"

func WorkspaceDownloadHttpError(statusCode any) string {
	return "下载失败（HTTP " + fmt.Sprint(statusCode) + "），可以换一个镜像再试"
}

const WorkspaceDownloadTimeout = "下载超时，请检查网络或换一个镜像"

func WorkspaceFfmpegFailed(e any) string {
	return "安装 ffmpeg 失败：" + fmt.Sprint(e)
}

const WorkspaceFfmpegInstalled = "已安装 ffmpeg。"

const WorkspaceInstallFfmpeg = "安装 ffmpeg"

func WorkspaceMirrorUnreachable(message any) string {
	return "无法连接镜像站：" + fmt.Sprint(message)
}

const WorkspacePreparing = "正在准备"

const WorkspaceStart = "开始准备"

const WorkspaceStepConfigure = "配置工作区"

const WorkspaceStepDownload = "下载 Ubuntu 基础系统"

const WorkspaceStepUnpack = "解压 Ubuntu 基础系统"

const WorkspaceTitle = "准备 Linux 工作区"

const WorkspaceUninstallNote = "需要联网。卸载应用会删除工作区和其中的课程库；可以先在「设置 → 数据与同步」导出备份。"

func WorkspaceUnpackFailed(msg any) string {
	return "解压失败：" + fmt.Sprint(msg)
}
