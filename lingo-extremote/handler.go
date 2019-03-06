package extremote

import (
	"errors"
	"fmt"

	"github.com/oandrew/ipod"

	"github.com/godbus/dbus"

	"log"
)

type DeviceExtRemote interface {
	PlaybackStatus() (trackLength, trackPos uint32, state PlayerState)
}

func ackSuccess(req *ipod.Command) *ACK {
	return &ACK{Status: ACKStatusSuccess, CmdID: req.ID.CmdID()}
}

// func ackPending(req ipod.Packet, maxWait uint32) ACKPending {
// 	return ACKPending{Status: ACKStatusPending, CmdID: uint8(req.ID.CmdID()), MaxWait: maxWait}
// }

func getTrackMetadata(connection *dbus.Conn, playerPath dbus.ObjectPath) (map[string]interface{}, error) {
	if playerPath == "" {
		return nil, errors.New("playerPath is empty")
	}

	player := connection.Object("org.bluez", playerPath)
	track, err := player.GetProperty("org.bluez.MediaPlayer1.Track")
	if err != nil {
		return nil, errors.New("could not get track from dbus: " + err.Error())
	}

	t, ok := track.Value().(map[string]dbus.Variant)
	if ok {
		return map[string]interface{}{"title": t["Title"].String(), "artist": t["Artist"].String()}, nil
	}
	return nil, errors.New("Could not coerce track to map")
}

func getPlayerStatus(connection *dbus.Conn, playerPath dbus.ObjectPath) (string, error) {
	if playerPath == "" {
		return "", errors.New("playerPath is empty")
	}

	player := connection.Object("org.bluez", playerPath)
	status, err := player.GetProperty("org.bluez.MediaPlayer1.Status")
	if err != nil {
		return "", errors.New("could not get player status from dbus: " + err.Error())
	}

	return status.Value().(string), nil
}

func HandleExtRemote(req *ipod.Command, tr ipod.CommandWriter, dev DeviceExtRemote) error {
	var playerPath dbus.ObjectPath
	conn, err := dbus.SystemBus()
	if err == nil {
		var objects map[dbus.ObjectPath]map[string]map[string]dbus.Variant
		conn.Object("org.bluez", "/").Call("org.freedesktop.DBus.ObjectManager.GetManagedObjects", 0).Store(&objects)

		for path, obj := range objects {
			for inter := range obj {
				if inter == "org.bluez.MediaPlayer1" {
					playerPath = path
					break
				}
			}
		}
	}

	log.Printf("playerPath: %v", playerPath)

	//log.Printf("Req: %#v", req)
	switch msg := req.Payload.(type) {

	case *GetCurrentPlayingTrackChapterInfo:
		ipod.Respond(req, tr, &ReturnCurrentPlayingTrackChapterInfo{
			CurrentChapterIndex: -1,
			ChapterCount:        0,
		})
	case *SetCurrentPlayingTrackChapter:
		ipod.Respond(req, tr, ackSuccess(req))
	case *GetCurrentPlayingTrackChapterPlayStatus:
		ipod.Respond(req, tr, &ReturnCurrentPlayingTrackChapterPlayStatus{
			ChapterPosition: 0,
			ChapterLength:   0,
		})
	case *GetCurrentPlayingTrackChapterName:
		ipod.Respond(req, tr, &ReturnCurrentPlayingTrackChapterName{
			ChapterName: ipod.StringToBytes("chapter"),
		})
	case *GetAudiobookSpeed:
		ipod.Respond(req, tr, &ReturnAudiobookSpeed{
			Speed: 0,
		})
	case *SetAudiobookSpeed:
		ipod.Respond(req, tr, ackSuccess(req))
	case *GetIndexedPlayingTrackInfo:
		var info interface{}
		switch msg.InfoType {
		case TrackInfoCaps:
			info = &TrackCaps{
				Caps:         0x0,
				TrackLength:  300 * 1000,
				ChapterCount: 0,
			}
		case TrackInfoDescription, TrackInfoLyrics:
			info = &TrackLongText{
				Flags:       0x0,
				PacketIndex: 0,
				Text:        0x00,
			}
		case TrackInfoGenre:
			info = ipod.StringToBytes("Genre")
		case TrackInfoComposer:
			info = ipod.StringToBytes("Composer")
		case TrackInfoArtworkCount:
			info = struct{}{}
		default:
			info = ipod.StringToBytes("WAT")
		}
		ipod.Respond(req, tr, &ReturnIndexedPlayingTrackInfo{
			InfoType: msg.InfoType,
			Info:     info,
		})
	case *GetArtworkFormats:
		ipod.Respond(req, tr, &RetArtworkFormats{})
	case *GetTrackArtworkData:
		ipod.Respond(req, tr, &ACK{
			Status: ACKStatusFailed,
			CmdID:  req.ID.CmdID(),
		})
	case *ResetDBSelection:
		ipod.Respond(req, tr, ackSuccess(req))
	case *SelectDBRecord:
		ipod.Respond(req, tr, ackSuccess(req))
	case *GetNumberCategorizedDBRecords:
		switch msg.CategoryType {
		case DbCategoryTrack:
			ipod.Respond(req, tr, &ReturnNumberCategorizedDBRecords{
				RecordCount: 10,
			})
		default:
			ipod.Respond(req, tr, &ReturnNumberCategorizedDBRecords{
				RecordCount: 0,
			})
		}
	case *RetrieveCategorizedDatabaseRecords:
		if msg.CategoryType == DbCategoryTrack {
			var arr [16]byte
			copy(arr[:], ipod.StringToBytes(fmt.Sprintf("Track %d", msg.Offset)))
			ipod.Respond(req, tr, &ReturnCategorizedDatabaseRecord{msg.Offset, arr})
			return nil
		}
		ipod.Respond(req, tr, &ReturnCategorizedDatabaseRecord{})
	case *GetPlayStatus:
		var state PlayerState
		status, err := getPlayerStatus(conn, playerPath)
		if err != nil {
			log.Printf("ERROR: getting play status from dbus: " + err.Error())
			state = PlayerStateStopped
		} else {
			log.Printf("play status from dbus: " + status)
			switch status {
			case "playing":
				state = PlayerStatePlaying
			default:
				state = PlayerStatePaused
			}
		}
		ipod.Respond(req, tr, &ReturnPlayStatus{
			State:         state,
			TrackIndex:    0,
			TrackLength:   300 * 1000,
			TrackPosition: 20 * 1000,
		})
	case *GetCurrentPlayingTrackIndex:
		ipod.Respond(req, tr, &ReturnCurrentPlayingTrackIndex{
			TrackIndex: 0,
		})
	case *GetIndexedPlayingTrackTitle:
		var title string
		track, err := getTrackMetadata(conn, playerPath)
		if err != nil {
			title = err.Error()
		} else {
			title = track["title"].(string)
		}
		ipod.Respond(req, tr, &ReturnIndexedPlayingTrackTitle{
			Title: ipod.StringToBytes(title),
		})
	case *GetIndexedPlayingTrackArtistName:
		var artist string
		track, err := getTrackMetadata(conn, playerPath)
		if err != nil {
			artist = err.Error()
		} else {
			artist = track["artist"].(string)
		}
		ipod.Respond(req, tr, &ReturnIndexedPlayingTrackArtistName{
			ArtistName: ipod.StringToBytes(artist),
		})
	case *GetIndexedPlayingTrackAlbumName:
		ipod.Respond(req, tr, &ReturnIndexedPlayingTrackAlbumName{
			AlbumName: ipod.StringToBytes("album"),
		})
	case *SetPlayStatusChangeNotification:
		ipod.Respond(req, tr, ackSuccess(req))
	case *SetPlayStatusChangeNotificationShort:
		ipod.Respond(req, tr, ackSuccess(req))
	case *PlayCurrentSelection:
		ipod.Respond(req, tr, ackSuccess(req))
	case *PlayControl:
		payload := req.Payload.(*PlayControl)
		switch payload.Cmd {
		case PlayControlToggle:
			status, err := getPlayerStatus(conn, playerPath)
			log.Printf("Play status from dbus: " + status)
			if err != nil {
				log.Printf("ERROR: getting play status from dbus: %s", err.Error())
				if playerPath != "" {
					err := conn.Object("org.bluez", playerPath).Call("org.bluez.MediaPlayer1.Play", 0).Store()
					if err != nil {
						log.Printf("ERROR: calling Play: %v", err)
					}
				}
			} else {
				switch status {
				case "playing":
					if playerPath != "" {
						err := conn.Object("org.bluez", playerPath).Call("org.bluez.MediaPlayer1.Pause", 0).Store()
						if err != nil {
							log.Printf("ERROR: calling Pause: %v", err)
						}
					}
				case "stopped":
					if playerPath != "" {
						err := conn.Object("org.bluez", playerPath).Call("org.bluez.MediaPlayer1.Play", 0).Store()
						if err != nil {
							log.Printf("ERROR: calling Pause: %v", err)
						}
					}
				case "paused":
					if playerPath != "" {
						err := conn.Object("org.bluez", playerPath).Call("org.bluez.MediaPlayer1.Play", 0).Store()
						if err != nil {
							log.Printf("ERROR: calling Pause: %v", err)
						}
					}
				default:
					if playerPath != "" {
						err := conn.Object("org.bluez", playerPath).Call("org.bluez.MediaPlayer1.Play", 0).Store()
						if err != nil {
							log.Printf("ERROR: calling Play: %v", err)
						}
					}
				}
			}
		case PlayControlPlay:
			log.Print("Play")
			if playerPath != "" {
				err := conn.Object("org.bluez", playerPath).Call("org.bluez.MediaPlayer1.Play", 0).Store()
				if err != nil {
					log.Printf("ERROR: calling Play: %v", err)
				}
			}
			return nil
		case PlayControlPause:
			log.Print("Pause")
			if playerPath != "" {
				err := conn.Object("org.bluez", playerPath).Call("org.bluez.MediaPlayer1.Pause", 0).Store()
				if err != nil {
					log.Printf("ERROR: calling Pause: %v", err)
				}
			}
			return nil
		case PlayControlNextTrack, PlayControlNextChapter, PlayControlNext:
			log.Print("Next")
			if playerPath != "" {
				err := conn.Object("org.bluez", playerPath).Call("org.bluez.MediaPlayer1.Next", 0).Store()
				if err != nil {
					log.Printf("ERROR: calling Next: %v", err)
				}
			}
			return nil
		case PlayControlPrevTrack, PlayControlPrevChapter, PlayControlPrev:
			log.Print("Prev")
			if playerPath != "" {
				err := conn.Object("org.bluez", playerPath).Call("org.bluez.MediaPlayer1.Previous", 0).Store()
				if err != nil {
					log.Printf("ERROR: calling Previous: %v", err)
				}
			}
			ipod.Respond(req, tr, &ACK{Status: ACKStatus(PlayControlPrevTrack), CmdID: req.ID.CmdID()})
			return nil
		}
		ipod.Respond(req, tr, ackSuccess(req))
	case *GetTrackArtworkTimes:
		ipod.Respond(req, tr, &RetTrackArtworkTimes{})
	case *GetShuffle:
		ipod.Respond(req, tr, &ReturnShuffle{Mode: ShuffleOff})
	case *SetShuffle:
		ipod.Respond(req, tr, ackSuccess(req))

	case *GetRepeat:
		ipod.Respond(req, tr, &ReturnRepeat{Mode: RepeatOff})
	case *SetRepeat:
		ipod.Respond(req, tr, ackSuccess(req))

	case *SetDisplayImage:
		ipod.Respond(req, tr, ackSuccess(req))
	case *GetMonoDisplayImageLimits:
		ipod.Respond(req, tr, &ReturnMonoDisplayImageLimits{
			MaxWidth:    640,
			MaxHeight:   960,
			PixelFormat: 0x01,
		})
	case *GetNumPlayingTracks:
		if playerPath != "" {
			ipod.Respond(req, tr, &ReturnNumPlayingTracks{
				NumTracks: 10,
			})
		} else {
			ipod.Respond(req, tr, &ReturnNumPlayingTracks{
				NumTracks: 0,
			})
		}
	case *SetCurrentPlayingTrack:
	case *SelectSortDBRecord:
	case *GetColorDisplayImageLimits:
		ipod.Respond(req, tr, &ReturnColorDisplayImageLimits{
			MaxWidth:    640,
			MaxHeight:   960,
			PixelFormat: 0x01,
		})
	case *ResetDBSelectionHierarchy:
		switch msg.Selection {
		case 1:
			ipod.Respond(req, tr, &ACK{Status: ACKStatusSuccess, CmdID: req.ID.CmdID()})
		default:
			ipod.Respond(req, tr, &ACK{Status: ACKStatusFailed, CmdID: req.ID.CmdID()})
		}

	case *GetDBiTunesInfo:
	// RetDBiTunesInfo:
	case *GetUIDTrackInfo:
	// RetUIDTrackInfo:
	case *GetDBTrackInfo:
	// RetDBTrackInfo:
	case *GetPBTrackInfo:
	// RetPBTrackInfo:

	default:
		_ = msg
	}
	return nil
}
