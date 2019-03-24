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
		return map[string]interface{}{"title": t["Title"].String(), "artist": t["Artist"].String(), "album": t["Album"].Value().(string)}, nil
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
		case TrackInfoArtistName:
			info = ipod.StringToBytes("GetIndexedPlayingTrackInfo ArtistName")
		case TrackInfoAlbum:
			info = ipod.StringToBytes("GetIndexedPlayingTrackInfo Album")
		case TrackInfoGenre:
			info = ipod.StringToBytes("GetIndexedPlayingTrackInfo Genre")
		case TrackInfoTitle:
			info = ipod.StringToBytes("GetIndexedPlayingTrackInfo Title")
		case TrackInfoComposer:
			info = ipod.StringToBytes("GetIndexedPlayingTrackInfo Composer")
		case TrackInfoArtworkCount:
			info = struct{}{}
		case TrackInfoLyrics:
			info = &TrackLongText{
				Flags:       0x0,
				PacketIndex: 0,
				Text:        0x00,
			}
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
		if playerPath == "" {
			ipod.Respond(req, tr, &ReturnCurrentPlayingTrackIndex{
				TrackIndex: -1,
			})
		} else {
			ipod.Respond(req, tr, &ReturnCurrentPlayingTrackIndex{
				TrackIndex: 0,
			})
		}
	case *GetIndexedPlayingTrackTitle:
		var title string
		track, err := getTrackMetadata(conn, playerPath)
		if err != nil {
			log.Printf("ERROR: getting track title: %s", err.Error())
			title = err.Error()
		} else {
			title = track["title"].(string)
			log.Printf("got track title: %s", title)
		}
		ipod.Respond(req, tr, &ReturnIndexedPlayingTrackTitle{
			Title: ipod.StringToBytes(title),
		})
	case *GetIndexedPlayingTrackArtistName:
		var artist string
		track, err := getTrackMetadata(conn, playerPath)
		if err != nil {
			log.Printf("ERROR: getting track artist: %s", err.Error())
			artist = err.Error()
		} else {
			artist = track["artist"].(string)
			log.Printf("got track artist: %s", artist)
		}
		ipod.Respond(req, tr, &ReturnIndexedPlayingTrackArtistName{
			ArtistName: ipod.StringToBytes(artist),
		})
	case *GetIndexedPlayingTrackAlbumName:
		var album string
		track, err := getTrackMetadata(conn, playerPath)
		if err != nil {
			log.Printf("ERROR: getting track album: %s", err.Error())
			album = err.Error()
		} else {
			album = track["album"].(string)
			log.Printf("got track album: %s", album)
		}
		ipod.Respond(req, tr, &ReturnIndexedPlayingTrackAlbumName{
			AlbumName: ipod.StringToBytes(album),
		})
	case *SetPlayStatusChangeNotification:
		ipod.Respond(req, tr, ackSuccess(req))
	case *SetPlayStatusChangeNotificationShort:
		ipod.Respond(req, tr, ackSuccess(req))
	case *PlayCurrentSelection:
		ipod.Respond(req, tr, ackSuccess(req))
	case *PlayControl:
		if playerPath == "" {
			break
		}

		payload := req.Payload.(*PlayControl)
		var err error

		switch payload.Cmd {
		case PlayControlToggle:
			status, err := getPlayerStatus(conn, playerPath)
			log.Printf("Play status from dbus: " + status)
			if err != nil {
				log.Printf("ERROR: getting play status from dbus: %s", err.Error())
				err := conn.Object("org.bluez", playerPath).Call("org.bluez.MediaPlayer1.Play", 0).Store()
				if err != nil {
					log.Printf("ERROR: calling Play: %v", err)
				}
				break
			}

			switch status {
			case "playing":
				log.Printf("current status is '%s' calling DBus '%s'", status, "Pause")
				err := conn.Object("org.bluez", playerPath).Call("org.bluez.MediaPlayer1.Pause", 0).Store()
				if err != nil {
					log.Printf("ERROR: calling Pause: %v", err)
				}
			case "stopped":
				log.Printf("current status is '%s' calling DBus '%s'", status, "Play")
				err := conn.Object("org.bluez", playerPath).Call("org.bluez.MediaPlayer1.Play", 0).Store()
				if err != nil {
					log.Printf("ERROR: calling Play: %v", err)
				}
			case "paused":
				log.Printf("current status is '%s' calling DBus '%s'", status, "Play")
				err := conn.Object("org.bluez", playerPath).Call("org.bluez.MediaPlayer1.Play", 0).Store()
				if err != nil {
					log.Printf("ERROR: calling Play: %v", err)
				}
			default:
				log.Printf("current status is '%s' calling DBus '%s'", status, "Play")
				err := conn.Object("org.bluez", playerPath).Call("org.bluez.MediaPlayer1.Play", 0).Store()
				if err != nil {
					log.Printf("ERROR: calling Play: %v", err)
				}
			}

		case PlayControlPlay:
			log.Print("Play")
			err := conn.Object("org.bluez", playerPath).Call("org.bluez.MediaPlayer1.Play", 0).Store()
			if err != nil {
				log.Printf("ERROR: calling Play: %v", err)
			}
		case PlayControlPause:
			log.Print("Pause")
			err := conn.Object("org.bluez", playerPath).Call("org.bluez.MediaPlayer1.Pause", 0).Store()
			if err != nil {
				log.Printf("ERROR: calling Pause: %v", err)
			}
		case PlayControlNextTrack, PlayControlNextChapter, PlayControlNext:
			log.Print("Next")
			err := conn.Object("org.bluez", playerPath).Call("org.bluez.MediaPlayer1.Next", 0).Store()
			if err != nil {
				log.Printf("ERROR: calling Next: %v", err)
			}
		case PlayControlPrevTrack, PlayControlPrevChapter, PlayControlPrev:
			log.Print("Prev")
			err := conn.Object("org.bluez", playerPath).Call("org.bluez.MediaPlayer1.Previous", 0).Store()
			if err != nil {
				log.Printf("ERROR: calling Previous: %v", err)
			}
		}

		if err != nil {
			log.Printf("ERROR: PlayControl (%v): %s", payload.Cmd, err.Error())
			ipod.Respond(req, tr, &ACK{Status: ACKStatusFailed, CmdID: req.ID.CmdID()})
		} else {
			// TODO: not sure if the ACK status should be 0 (success) or the actual control status?
			log.Printf("Responding with ACKStatus: %v (%d)", ACKStatus(payload.Cmd), payload.Cmd)
			// ipod.Respond(req, tr, &ACK{Status: ACKStatusSuccess, CmdID: req.ID.CmdID()})
			ipod.Respond(req, tr, &ACK{Status: ACKStatus(payload.Cmd), CmdID: req.ID.CmdID()})
		}

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
