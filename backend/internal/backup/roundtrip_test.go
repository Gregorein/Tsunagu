package backup_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"tsunagu/backend/internal/backup"
	tdb "tsunagu/backend/internal/db"
	"tsunagu/backend/internal/db/sqlcgen"
)

func openTestDB(t *testing.T) *sqlcgen.Queries {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	conn, err := tdb.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return sqlcgen.New(conn)
}

func mustCreateExtension(t *testing.T, q *sqlcgen.Queries, pkg string, sourceID int64) sqlcgen.Extension {
	t.Helper()
	ctx := context.Background()
	repo, err := q.CreateRepository(ctx, sqlcgen.CreateRepositoryParams{
		IndexUrl:    "https://example.com/" + pkg,
		Name:        sql.NullString{String: pkg, Valid: true},
		ContentType: "manga",
	})
	if err != nil {
		t.Fatalf("create repository: %v", err)
	}
	ext, err := q.UpsertExtension(ctx, sqlcgen.UpsertExtensionParams{
		RepositoryID: repo.ID,
		PackageName:  pkg,
		Name:         "Test Source",
		Version:      "1.0.0",
		ContentType:  "manga",
		Lang:         "en",
		ApkUrl:       "https://example.com/" + pkg + ".apk",
	})
	if err != nil {
		t.Fatalf("upsert extension: %v", err)
	}
	updated, err := q.UpdateExtensionSourceID(ctx, sqlcgen.UpdateExtensionSourceIDParams{SourceID: sourceID, ID: ext.ID})
	if err != nil {
		t.Fatalf("set source id: %v", err)
	}
	installed, err := q.MarkExtensionInstalled(ctx, sqlcgen.MarkExtensionInstalledParams{ID: updated.ID})
	if err != nil {
		t.Fatalf("mark installed: %v", err)
	}
	return installed
}

func TestExportImportRoundTrip(t *testing.T) {
	ctx := context.Background()
	srcQ := openTestDB(t)

	ext := mustCreateExtension(t, srcQ, "com.example.test", 123456789)

	media, err := srcQ.UpsertMediaDetails(ctx, sqlcgen.UpsertMediaDetailsParams{
		ExtensionID:   sql.NullInt64{Int64: ext.ID, Valid: true},
		ExtensionName: ext.Name,
		ExternalID:    "/manga/one-piece",
		ContentType:   "manga",
		Title:         "One Piece",
		Status:        sql.NullString{String: "Ongoing", Valid: true},
		Author:        sql.NullString{String: "Oda", Valid: true},
	})
	if err != nil {
		t.Fatalf("upsert media: %v", err)
	}
	if _, err := srcQ.AddMediaToLibrary(ctx, media.ID); err != nil {
		t.Fatalf("add to library: %v", err)
	}

	folder, err := srcQ.CreateFolder(ctx, sqlcgen.CreateFolderParams{Name: "Favorites"})
	if err != nil {
		t.Fatalf("create folder: %v", err)
	}
	if err := srcQ.AddMediaToFolder(ctx, sqlcgen.AddMediaToFolderParams{MediaID: media.ID, FolderID: folder.ID}); err != nil {
		t.Fatalf("add to folder: %v", err)
	}

	chapter, err := srcQ.CreateChapter(ctx, sqlcgen.CreateChapterParams{
		MediaID:    media.ID,
		ExternalID: "/manga/one-piece/1",
		Title:      sql.NullString{String: "Romance Dawn", Valid: true},
		Number:     sql.NullFloat64{Float64: 1, Valid: true},
	})
	if err != nil {
		t.Fatalf("create chapter: %v", err)
	}
	if _, err := srcQ.UpsertReadingProgress(ctx, sqlcgen.UpsertReadingProgressParams{
		MediaID:   media.ID,
		ChapterID: chapter.ID,
		Progress:  1,
		Completed: true,
	}); err != nil {
		t.Fatalf("mark chapter read: %v", err)
	}

	acct, err := srcQ.UpsertTrackerAccount(ctx, sqlcgen.UpsertTrackerAccountParams{
		TrackerType: "anilist",
		AccessToken: "token",
		Username:    "tester",
	})
	if err != nil {
		t.Fatalf("upsert tracker account: %v", err)
	}
	if _, err := srcQ.UpsertTrackerLink(ctx, sqlcgen.UpsertTrackerLinkParams{
		MediaID:           media.ID,
		TrackerAccountID:  acct.ID,
		ExternalTrackerID: "999",
		TrackerTitle:      "One Piece",
		RemoteUrl:         "https://anilist.co/manga/999",
		Status:            1,
		LastChapterRead:   1,
		SyncProgress:      true,
	}); err != nil {
		t.Fatalf("upsert tracker link: %v", err)
	}

	b, err := backup.Export(ctx, srcQ)
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	filePath := filepath.Join(t.TempDir(), "test.tachibk")
	if err := backup.WriteMihonFile(filePath, b); err != nil {
		t.Fatalf("write mihon file: %v", err)
	}
	b, err = backup.ReadMihonFile(filePath)
	if err != nil {
		t.Fatalf("read mihon file: %v", err)
	}

	if len(b.Manga) != 1 {
		t.Fatalf("expected 1 manga in export, got %d", len(b.Manga))
	}
	if b.Manga[0].Source != 123456789 {
		t.Fatalf("expected source 123456789, got %d", b.Manga[0].Source)
	}
	if len(b.Manga[0].Chapters) != 1 || !b.Manga[0].Chapters[0].Read {
		t.Fatalf("expected 1 read chapter in export, got %+v", b.Manga[0].Chapters)
	}
	if len(b.Manga[0].Tracking) != 1 || b.Manga[0].Tracking[0].SyncId != 2 {
		t.Fatalf("expected 1 anilist(syncId=2) tracking entry, got %+v", b.Manga[0].Tracking)
	}
	if len(b.Categories) != 1 || b.Categories[0].Name != "Favorites" {
		t.Fatalf("expected 1 category %q, got %+v", "Favorites", b.Categories)
	}

	dstQ := openTestDB(t)
	dstExt := mustCreateExtension(t, dstQ, "com.example.test", 123456789)
	if _, err := dstQ.UpsertTrackerAccount(ctx, sqlcgen.UpsertTrackerAccountParams{
		TrackerType: "anilist",
		AccessToken: "token2",
		Username:    "tester2",
	}); err != nil {
		t.Fatalf("upsert tracker account on dst: %v", err)
	}

	res, err := backup.Import(ctx, dstQ, b)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if res.MangaImported != 1 || res.MangaSkipped != 0 {
		t.Fatalf("unexpected import result: %+v", res)
	}
	if res.TrackingImported != 1 {
		t.Fatalf("expected 1 tracking entry imported, got %d", res.TrackingImported)
	}

	imported, err := dstQ.GetMediaByExtensionAndExternalID(ctx, sqlcgen.GetMediaByExtensionAndExternalIDParams{
		ExtensionID: sql.NullInt64{Int64: dstExt.ID, Valid: true},
		ExternalID:  "/manga/one-piece",
	})
	if err != nil {
		t.Fatalf("lookup imported media: %v", err)
	}
	if imported.Title != "One Piece" || !imported.AddedAt.Valid {
		t.Fatalf("imported media not favorited/titled correctly: %+v", imported)
	}

	chapters, err := dstQ.ListChaptersByMedia(ctx, imported.ID)
	if err != nil || len(chapters) != 1 {
		t.Fatalf("expected 1 imported chapter, got %d (err=%v)", len(chapters), err)
	}
	progress, err := dstQ.ListReadingProgressByMedia(ctx, imported.ID)
	if err != nil || len(progress) != 1 || !progress[0].Completed {
		t.Fatalf("expected imported chapter to be marked read, got %+v (err=%v)", progress, err)
	}

	folders, err := dstQ.ListFoldersByMediaIDs(ctx, []int64{imported.ID})
	if err != nil || len(folders) != 1 || folders[0].Name != "Favorites" {
		t.Fatalf("expected imported media in Favorites folder, got %+v (err=%v)", folders, err)
	}
}
