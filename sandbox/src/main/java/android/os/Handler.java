package android.os;

import java.util.concurrent.Executors;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.TimeUnit;

public class Handler {

    public interface Callback {
        boolean handleMessage(Message msg);
    }

    private static final ScheduledExecutorService EXEC =
            Executors.newSingleThreadScheduledExecutor(r -> {
                Thread t = new Thread(r, "sandbox-handler");
                t.setDaemon(true);
                return t;
            });

    private final Callback callback;

    public Handler() {
        this.callback = null;
    }

    public Handler(Looper looper) {
        this.callback = null;
    }

    public Handler(Looper looper, Callback callback) {
        this.callback = callback;
    }

    public Handler(Callback callback) {
        this.callback = callback;
    }

    public final boolean post(Runnable r) {
        if (r != null) EXEC.execute(r);
        return true;
    }

    public final boolean postDelayed(Runnable r, long delayMillis) {
        if (r != null) EXEC.schedule(r, Math.max(0L, delayMillis), TimeUnit.MILLISECONDS);
        return true;
    }

    public final boolean postAtFrontOfQueue(Runnable r) {
        return post(r);
    }

    public final void removeCallbacks(Runnable r) {}

    public final void removeCallbacks(Runnable r, Object token) {}

    public final void removeCallbacksAndMessages(Object token) {}

    public final boolean sendMessage(Message msg) {
        return dispatch(msg);
    }

    public final boolean sendMessageDelayed(Message msg, long delayMillis) {
        return dispatch(msg);
    }

    public final boolean sendEmptyMessage(int what) {
        Message m = new Message();
        m.what = what;
        return dispatch(m);
    }

    public final boolean sendEmptyMessageDelayed(int what, long delayMillis) {
        return sendEmptyMessage(what);
    }

    public final boolean hasMessages(int what) {
        return false;
    }

    public void handleMessage(Message msg) {}

    public Looper getLooper() {
        return Looper.getMainLooper();
    }

    private boolean dispatch(Message msg) {
        EXEC.execute(() -> {
            if (callback == null || !callback.handleMessage(msg)) {
                handleMessage(msg);
            }
        });
        return true;
    }
}
