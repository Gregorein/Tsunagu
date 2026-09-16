-- "mature" was blocking moderate-level browsing for shows like Demon Slayer
-- that are tagged mature for violence, not sexual content. Move it to
-- strict-only; ecchi/hentai/adult/nsfw/etc. still block at moderate.
UPDATE content_filter_rules
SET block_level = 2
WHERE is_default = 1 AND category = 'sexual' AND field = 'genre' AND keyword = 'mature';
