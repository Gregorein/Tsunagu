UPDATE extensions SET lang = lower(trim(lang)) WHERE lang <> lower(trim(lang));
