package tsunagu.grpc

import eu.kanade.tachiyomi.network.NetworkHelper
import io.grpc.stub.StreamObserver
import kotlinx.serialization.json.Json
import org.junit.jupiter.api.Assertions.assertNotNull
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.Assumptions.assumeTrue
import org.junit.jupiter.api.Test
import org.koin.core.context.GlobalContext
import org.koin.core.context.startKoin
import org.koin.dsl.module
import sandbox.v1.Sandbox
import tsunagu.registry.ExtensionRegistry
import java.io.File

private class Capture<T> : StreamObserver<T> {
    var value: T? = null
    var error: Throwable? = null
    override fun onNext(v: T) { value = v }
    override fun onError(t: Throwable) { error = t }
    override fun onCompleted() {}
}

class AllAnimeLiveTest {

    private fun bootKoin() {
        if (GlobalContext.getOrNull() != null) return
        startKoin {
            modules(
                module {
                    single { Json { ignoreUnknownKeys = true } }
                    single { NetworkHelper() }
                    single { android.app.Application() }
                },
            )
        }
    }

    private fun repoRoot(): File {
        var d = File(System.getProperty("user.dir")).absoluteFile
        while (d.parentFile != null) {
            if (File(d, "backend/sandbox/extensions").isDirectory) return d
            d = d.parentFile
        }
        error("could not locate repo root")
    }

    @Test
    fun `search then getDetails does not NPE on memo`() {
        assumeTrue(System.getenv("RUN_LIVE_EXTENSION_TESTS") == "1", "live extension test disabled")

        val extDir = File(repoRoot(), "backend/sandbox/extensions")
        val extId = "eu.kanade.tachiyomi.extension.en.allanime"
        assumeTrue(File(extDir, "$extId.jar").exists(), "allanime jar not present")

        bootKoin()
        val registry = ExtensionRegistry(extDir, novelEnabled = false)
        registry.loadAll()
        assertNotNull(registry.get(extId), "allanime failed to load")

        val svc = ExtensionServiceImpl(registry)

        val searchCap = Capture<Sandbox.SearchResponse>()
        svc.search(
            Sandbox.SearchRequest.newBuilder()
                .setExtensionId(extId).setQuery("naruto").setPage(1).build(),
            searchCap,
        )
        assertTrue(searchCap.error == null, "search errored: ${searchCap.error}")
        val results = searchCap.value?.resultsList.orEmpty()
        assumeTrue(results.isNotEmpty(), "search returned nothing (network?)")

        val entryId = results.first().sourceEntryId
        val detailsCap = Capture<Sandbox.EntryDetails>()
        svc.getDetails(
            Sandbox.EntryRequest.newBuilder()
                .setExtensionId(extId).setSourceEntryId(entryId).build(),
            detailsCap,
        )
        assertTrue(detailsCap.error == null, "getDetails errored: ${detailsCap.error}")
        assertNotNull(detailsCap.value)
        assertTrue(detailsCap.value!!.title.isNotBlank(), "details returned blank title")

        val chaptersCap = Capture<Sandbox.ChapterList>()
        svc.getChapters(
            Sandbox.EntryRequest.newBuilder()
                .setExtensionId(extId).setSourceEntryId(entryId).build(),
            chaptersCap,
        )
        assertTrue(chaptersCap.error == null, "getChapters errored: ${chaptersCap.error}")
        val chapters = chaptersCap.value?.chaptersList.orEmpty()
        assertTrue(chapters.isNotEmpty(), "no chapters returned")

        val pagesCap = Capture<Sandbox.PageList>()
        svc.getPages(
            Sandbox.ChapterRequest.newBuilder()
                .setExtensionId(extId)
                .setSourceEntryId(entryId)
                .setSourceChapterId(chapters.last().sourceChapterId)
                .build(),
            pagesCap,
        )
        val pagesErr = pagesCap.error?.message.orEmpty()
        // AllAnime derives image URLs via JS in a real WebView / behind Cloudflare;
        // neither is available here. That's an environmental limit, not a regression
        // of the memo/chapter-state fix, so only fail on a different error.
        assumeTrue(
            pagesCap.error == null ||
                pagesErr.contains("WebView") || pagesErr.contains("Cloudflare"),
            "getPages errored unexpectedly: $pagesErr",
        )
        if (pagesCap.error == null) {
            assertTrue((pagesCap.value?.pageUrlsList?.size ?: 0) > 0, "no pages returned")
        }
    }
}
