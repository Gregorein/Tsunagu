package android.os;

public final class Message {

    public int what;
    public int arg1;
    public int arg2;
    public Object obj;

    public Message() {}

    public static Message obtain() {
        return new Message();
    }

    public static Message obtain(Handler h) {
        return new Message();
    }

    public static Message obtain(Handler h, int what) {
        Message m = new Message();
        m.what = what;
        return m;
    }

    public void sendToTarget() {}

    public void recycle() {}
}
