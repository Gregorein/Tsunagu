@file:OptIn(kotlinx.coroutines.InternalCoroutinesApi::class)

package tsunagu.compat

import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.MainCoroutineDispatcher
import kotlinx.coroutines.asCoroutineDispatcher
import kotlinx.coroutines.internal.MainDispatcherFactory
import java.util.concurrent.Executors
import kotlin.coroutines.CoroutineContext

class JvmMainDispatcherFactory : MainDispatcherFactory {
    override val loadPriority: Int = 0

    override fun createDispatcher(allFactories: List<MainDispatcherFactory>): MainCoroutineDispatcher =
        JvmMainDispatcher

    override fun hintOnError(): String = "Tsunagu sandbox has no UI thread; Dispatchers.Main is emulated on a background thread."
}

private object JvmMainDispatcher : MainCoroutineDispatcher() {
    private val delegate: CoroutineDispatcher =
        Executors.newSingleThreadExecutor { r ->
            Thread(r, "tsunagu-main").apply { isDaemon = true }
        }.asCoroutineDispatcher()

    override val immediate: MainCoroutineDispatcher get() = this

    override fun dispatch(context: CoroutineContext, block: Runnable) = delegate.dispatch(context, block)
}
