package desync

const (
	// Format identifiers used in archive files
	CaFormatEntry             = 0x1396fabcea5bbb51
	CaFormatUser              = 0xf453131aaeeaccb3
	CaFormatGroup             = 0x25eb6ac969396a52
	CaFormatXAttr             = 0xb8157091f80bc486
	CaFormatACLUser           = 0x297dc88b2ef12faf
	CaFormatACLGroup          = 0x36f2acb56cb3dd0b
	CaFormatACLGroupObj       = 0x23047110441f38f3
	CaFormatACLDefault        = 0xfe3eeda6823c8cd0
	CaFormatACLDefaultUser    = 0xbdf03df9bd010a91
	CaFormatACLDefaultGroup   = 0xa0cb1168782d1f51
	CaFormatFCaps             = 0xf7267db0afed0629
	CaFormatSELinux           = 0x46faf0602fd26c59
	CaFormatSymlink           = 0x664a6fb6830e0d6c
	CaFormatDevice            = 0xac3dace369dfe643
	CaFormatPayload           = 0x8b9e1d93d6dcffc9
	CaFormatFilename          = 0x6dbb6ebcb3161f0b
	CaFormatGoodbye           = 0xdfd35c5e8327c403
	CaFormatGoodbyeTailMarker = 0x57446fa533702943
	CaFormatIndex             = 0x96824d9c7b129ff9
	CaFormatTable             = 0xe75b9e112f17417d
	CaFormatTableTailMarker   = 0x4b4f050e5549ecd1

	// SipHash key used in Goodbye elements to hash the filename. It's 16 bytes,
	// split into 2x64bit values, upper and lower part of the key
	CaFormatGoodbyeHashKey0 = 0x8574442b0f1d84b3
	CaFormatGoodbyeHashKey1 = 0x2736ed30d1c22ec1

	// Format feature flags
	CaFormatWith16BitUIDs   = 0x1
	CaFormatWith32BitUIDs   = 0x2
	CaFormatWithUserNames   = 0x4
	CaFormatWithSecTime     = 0x8
	CaFormatWithUSecTime    = 0x10
	CaFormatWithNSecTime    = 0x20
	CaFormatWith2SecTime    = 0x40
	CaFormatWithReadOnly    = 0x80
	CaFormatWithPermissions = 0x100
	CaFormatWithSymlinks    = 0x200
	CaFormatWithDeviceNodes = 0x400
	CaFormatWithFIFOs       = 0x800
	CaFormatWithSockets     = 0x1000

	/* DOS file flags */
	CaFormatWithFlagHidden  = 0x2000
	CaFormatWithFlagSystem  = 0x4000
	CaFormatWithFlagArchive = 0x8000

	/* chattr() flags */
	CaFormatWithFlagAppend         = 0x10000
	CaFormatWithFlagNoAtime        = 0x20000
	CaFormatWithFlagCompr          = 0x40000
	CaFormatWithFlagNoCow          = 0x80000
	CaFormatWithFlagNoDump         = 0x100000
	CaFormatWithFlagDirSync        = 0x200000
	CaFormatWithFlagImmutable      = 0x400000
	CaFormatWithFlagSync           = 0x800000
	CaFormatWithFlagNoComp         = 0x1000000
	CaFormatWithFlagProjectInherit = 0x2000000

	/* btrfs magic */
	CaFormatWithSubvolume   = 0x4000000
	CaFormatWithSubvolumeRO = 0x8000000

	/* Extended Attribute metadata */
	CaFormatWithXattrs  = 0x10000000
	CaFormatWithACL     = 0x20000000
	CaFormatWithSELinux = 0x40000000
	CaFormatWithFcaps   = 0x80000000

	CaFormatExcludeFile      = 0x1000000000000000
	CaFormatSHA512256        = 0x2000000000000000
	CaFormatExcludeSubmounts = 0x4000000000000000
	CaFormatExcludeNoDump    = 0x8000000000000000

	// Protocol message types
	CaProtocolHello      = 0x3c71d0948ca5fbee
	CaProtocolIndex      = 0xb32a91dd2b3e27f8
	CaProtocolIndexEOF   = 0x4f0932f1043718f5
	CaProtocolArchive    = 0x95d6428a69eddcc5
	CaProtocolArchiveEOF = 0x450bef663f24cbad
	CaProtocolRequest    = 0x8ab427e0f89d9210
	CaProtocolChunk      = 0x5213dd180a84bc8c
	CaProtocolMissing    = 0xd010f9fac82b7b6c
	CaProtocolGoodbye    = 0xad205dbf1a3686c3
	CaProtocolAbort      = 0xe7d9136b7efea352

	// Provided services
	CaProtocolReadableStore   = 0x1
	CaProtocolWritableStore   = 0x2
	CaProtocolReadableIndex   = 0x4
	CaProtocolWritableIndex   = 0x8
	CaProtocolReadableArchive = 0x10
	CaProtocolWritableArchive = 0x20

	// Wanted services
	CaProtocolPullChunks      = 0x40
	CaProtocolPullIndex       = 0x80
	CaProtocolPullArchive     = 0x100
	CaProtocolPushChunks      = 0x200
	CaProtocolPushIndex       = 0x400
	CaProtocolPushIndexChunks = 0x800
	CaProtocolPushArchive     = 0x1000

	// Protocol request flags
	CaProtocolRequestHighPriority = 1

	// Chunk properties
	CaProtocolChunkCompressed = 1
)

var (
	FormatString = map[uint64]string{
		CaFormatEntry:             "CaFormatEntry",
		CaFormatUser:              "CaFormatUser",
		CaFormatGroup:             "CaFormatGroup",
		CaFormatXAttr:             "CaFormatXAttr",
		CaFormatACLUser:           "CaFormatACLUser",
		CaFormatACLGroup:          "CaFormatACLGroup",
		CaFormatACLGroupObj:       "CaFormatACLGroupObj",
		CaFormatACLDefault:        "CaFormatACLDefault",
		CaFormatACLDefaultUser:    "CaFormatACLDefaultUser",
		CaFormatACLDefaultGroup:   "CaFormatACLDefaultGroup",
		CaFormatFCaps:             "CaFormatFCaps",
		CaFormatSELinux:           "CaFormatSELinux",
		CaFormatSymlink:           "CaFormatSymlink",
		CaFormatDevice:            "CaFormatDevice",
		CaFormatPayload:           "CaFormatPayload",
		CaFormatFilename:          "CaFormatFilename",
		CaFormatGoodbye:           "CaFormatGoodbye",
		CaFormatGoodbyeTailMarker: "CaFormatGoodbyeTailMarker",
		CaFormatIndex:             "CaFormatIndex",
		CaFormatTable:             "CaFormatTable",
		CaFormatTableTailMarker:   "CaFormatTableTailMarker",
	}
)
