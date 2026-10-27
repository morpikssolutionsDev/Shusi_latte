import { api, unwrap, type Principal } from '@sushi-latte/api-client';
import { Button, Card, Chip, Dialog, Field, Input, Select, Switch, cn, imageUrl, videoUrl, wideImageUrl } from '@sushi-latte/ui';
import { useQuery } from '@tanstack/react-query';
import { ArrowDownIcon, ArrowUpIcon, ExternalLinkIcon, FilmIcon, ImagePlusIcon, PencilIcon, PlusIcon, Trash2Icon } from 'lucide-react';
import { useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { catalogQuery, discountsQuery, errorText, playlistQuery, slidesQuery, uploadImage, uploadVideo, useSave, type Slide } from './api';
import { PageHeader } from './layout';

const TEMPLATES = ['hero_promo', 'featured_product', 'happy_hour', 'category_spotlight', 'brand_idle', 'uploaded'] as const;
const needsImage = (t: Slide['template']) => t === 'hero_promo' || t === 'uploaded' || t === 'brand_idle' || t === 'category_spotlight';

const blank = (template: Slide['template']): Slide => ({
  template, active: true, headline: { id: '', en: '' }, subline: { id: '', en: '' }, imageFocus: { x: 50, y: 50 },
  categoryIds: [], steam: false, durationSeconds: 8,
});

/** Product Display playlist (BR-ADM-10). Management edits; Outlet Admin views. */
export function DisplayPage({ me }: { me: Principal }) {
  const { t } = useTranslation();
  const { data: slides } = useQuery(slidesQuery);
  const { data: catalog } = useQuery(catalogQuery);
  const { data: discounts } = useQuery(discountsQuery);
  const { data: playlist } = useQuery(playlistQuery);
  const playing = new Set(playlist?.slides.map((s) => s.id));
  const [edit, setEdit] = useState<Slide | null>(null);
  const [remove, setRemove] = useState<Slide | null>(null);
  const canEdit = me.role === 'management';
  const save = useSave(
    async (s: Slide) => (s.id ? unwrap(await api.PUT('/display/slides/{id}', { params: { path: { id: s.id } }, body: s })) : unwrap(await api.POST('/display/slides', { body: s }))),
    [['slides']],
    t('admin.common.saved'),
  );
  const del = useSave(async (id: string) => unwrap(await api.DELETE('/display/slides/{id}', { params: { path: { id } } })), [['slides']], t('admin.display.deleted'));
  const reorder = useSave(async (ids: string[]) => unwrap(await api.PUT('/display/slide-order', { body: { ids } })), [['slides']]);
  if (!slides || !catalog) return null;

  const productName = (id?: string) => catalog.products.find((p) => p.id === id)?.nameId;
  const move = (i: number, dir: -1 | 1) => {
    const ids = slides.map((s) => s.id!);
    const j = i + dir;
    [ids[i], ids[j]] = [ids[j]!, ids[i]!];
    reorder.mutate(ids);
  };
  const thumb = (s: Slide) => {
    const key = s.imageKey ?? catalog.products.find((p) => p.id === s.productId)?.imageKey;
    if (!key) return null;
    return s.imageKey ? wideImageUrl(key, s.imageFocus) : imageUrl(key, 'sq320');
  };

  return (
    <>
      <PageHeader
        title={t('admin.display.title')}
        actions={
          <>
            <Button variant="secondary" asChild>
              <a href={import.meta.env.VITE_DISPLAY_URL ?? '/display/'} target="_blank" rel="noreferrer">
                <ExternalLinkIcon aria-hidden />
                {t('admin.display.open')}
              </a>
            </Button>
            {canEdit && (
              <Button onClick={() => setEdit(blank('featured_product'))}>
                <PlusIcon aria-hidden />
                {t('admin.display.add')}
              </Button>
            )}
          </>
        }
      >
        {t('admin.display.hint')}
      </PageHeader>
      <ol className="flex flex-col gap-3">
        {slides.map((s, i) => {
          const src = thumb(s);
          return (
            <li key={s.id}>
              <Card className={cn('flex items-center gap-4 p-3', !s.active && 'opacity-60')}>
                <span className="tabular w-6 text-center text-sm font-bold text-muted-foreground">{i + 1}</span>
                <div className="aspect-video w-36 shrink-0 overflow-hidden rounded-lg bg-[var(--sl-color-nori-900)]">
                  {!src && s.videoKey && <video src={videoUrl(s.videoKey)} muted preload="metadata" className="size-full object-cover" />}
                  {src && <img src={src} alt="" className="size-full object-cover" onError={(e) => ((e.currentTarget as HTMLImageElement).style.visibility = 'hidden')} />}
                </div>
                <div className="min-w-0 flex-1">
                  <p className="truncate font-semibold">{s.headline.id || productName(s.productId) || t(`admin.display.templates.${s.template}`)}</p>
                  <div className="mt-1 flex flex-wrap items-center gap-1.5">
                    <Chip feedback="info">{t(`admin.display.templates.${s.template}`)}</Chip>
                    {s.productId && <span className="text-xs text-muted-foreground">{productName(s.productId)}</span>}
                    <span className="tabular text-xs text-muted-foreground">{s.durationSeconds} dtk</span>
                    {playlist && s.active && !playing.has(s.id!) && <Chip feedback="warning">{t('admin.display.skipped')}</Chip>}
                  </div>
                </div>
                {canEdit && (
                  <div className="flex items-center gap-1">
                    <Switch aria-label={`${t('admin.common.active')} ${i + 1}`} checked={s.active} onCheckedChange={(active) => save.mutate({ ...s, active })} />
                    <Button variant="ghost" size="icon" aria-label={t('admin.display.up')} disabled={i === 0} onClick={() => move(i, -1)}><ArrowUpIcon aria-hidden /></Button>
                    <Button variant="ghost" size="icon" aria-label={t('admin.display.down')} disabled={i === slides.length - 1} onClick={() => move(i, 1)}><ArrowDownIcon aria-hidden /></Button>
                    <Button variant="ghost" size="icon" aria-label={t('admin.common.edit')} onClick={() => setEdit(structuredClone(s))}><PencilIcon aria-hidden /></Button>
                    <Button variant="ghost" size="icon" aria-label={t('admin.common.remove')} onClick={() => setRemove(s)}><Trash2Icon aria-hidden /></Button>
                  </div>
                )}
              </Card>
            </li>
          );
        })}
      </ol>

      {edit && (
        <SlideEditor
          slide={edit}
          onChange={setEdit}
          onClose={() => setEdit(null)}
          onSave={() => save.mutate(edit, { onSuccess: () => setEdit(null) })}
          saving={save.isPending}
          products={catalog.products.filter((p) => p.status !== 'archived')}
          categories={catalog.categories}
          discounts={(discounts ?? []).filter((d) => d.trigger === 'auto')}
        />
      )}
      {remove && (
        <Dialog open onOpenChange={(o) => !o && setRemove(null)} title={t('admin.display.deleteTitle')}
          footer={<>
            <Button variant="secondary" onClick={() => setRemove(null)}>{t('admin.common.cancel')}</Button>
            <Button variant="danger" onClick={() => del.mutate(remove.id!, { onSuccess: () => setRemove(null) })}>{t('admin.common.remove')}</Button>
          </>}>
          <p>{remove.headline.id || t(`admin.display.templates.${remove.template}`)}</p>
        </Dialog>
      )}
    </>
  );
}

function SlideEditor({ slide: s, onChange, onClose, onSave, saving, products, categories, discounts }: {
  slide: Slide; onChange: (s: Slide) => void; onClose: () => void; onSave: () => void; saving: boolean;
  products: { id?: string; nameId: string }[]; categories: { id?: string; nameId: string }[]; discounts: { id?: string; name: string }[];
}) {
  const { t } = useTranslation();
  const file = useRef<HTMLInputElement>(null);
  const [uploading, setUploading] = useState(false);
  const set = (patch: Partial<Slide>) => onChange({ ...s, ...patch });
  const video = useRef<HTMLInputElement>(null);
  const upload = async (f: File) => {
    setUploading(true);
    try {
      set(f.type.startsWith('video/') ? { videoKey: await uploadVideo(f) } : { imageKey: await uploadImage(f), imageFocus: { x: 50, y: 50 } });
    } catch (e) {
      toast.error(e instanceof Error ? e.message : errorText(e));
    } finally {
      setUploading(false);
    }
  };
  const hasProduct = s.template === 'featured_product' || s.template === 'happy_hour';

  return (
    <Dialog open variant="sheet" onOpenChange={(o) => !o && onClose()} title={s.id ? t('admin.common.edit') : t('admin.display.add')}
      footer={<>
        <Button variant="secondary" onClick={onClose}>{t('admin.common.cancel')}</Button>
        <Button disabled={saving || uploading} onClick={onSave}>{t('admin.common.save')}</Button>
      </>}>
      <div className="grid gap-4 sm:grid-cols-2">
        <Field label={t('admin.display.template')} htmlFor="s-template" className="sm:col-span-2" hint={t(`admin.display.templateHints.${s.template}`)}>
          <Select id="s-template" value={s.template} onChange={(e) => set({ template: e.target.value as Slide['template'] })}>
            {TEMPLATES.map((x) => <option key={x} value={x}>{t(`admin.display.templates.${x}`)}</option>)}
          </Select>
        </Field>
        <Field label={t('admin.display.headlineId')} htmlFor="s-hid"><Input id="s-hid" value={s.headline.id} onChange={(e) => set({ headline: { ...s.headline, id: e.target.value } })} /></Field>
        <Field label={t('admin.display.headlineEn')} htmlFor="s-hen"><Input id="s-hen" value={s.headline.en} onChange={(e) => set({ headline: { ...s.headline, en: e.target.value } })} /></Field>
        <Field label={t('admin.display.sublineId')} htmlFor="s-sid"><Input id="s-sid" value={s.subline.id} onChange={(e) => set({ subline: { ...s.subline, id: e.target.value } })} /></Field>
        <Field label={t('admin.display.sublineEn')} htmlFor="s-sen"><Input id="s-sen" value={s.subline.en} onChange={(e) => set({ subline: { ...s.subline, en: e.target.value } })} /></Field>
        {hasProduct && (
          <Field label={t('admin.display.product')} htmlFor="s-product">
            <Select id="s-product" value={s.productId ?? ''} onChange={(e) => set({ productId: e.target.value || undefined })}>
              <option value="">—</option>
              {products.map((p) => <option key={p.id} value={p.id}>{p.nameId}</option>)}
            </Select>
          </Field>
        )}
        {s.template === 'happy_hour' && (
          <Field label={t('admin.display.discount')} htmlFor="s-discount">
            <Select id="s-discount" value={s.discountId ?? ''} onChange={(e) => set({ discountId: e.target.value || undefined })}>
              <option value="">—</option>
              {discounts.map((d) => <option key={d.id} value={d.id}>{d.name}</option>)}
            </Select>
          </Field>
        )}
        {s.template === 'category_spotlight' && (
          <fieldset className="sm:col-span-2">
            <legend className="mb-1.5 text-sm font-semibold">{t('admin.nav.categories')}</legend>
            <div className="flex flex-wrap gap-1.5">
              {categories.map((c) => {
                const on = s.categoryIds.includes(c.id!);
                return (
                  <button key={c.id} type="button" aria-pressed={on}
                    onClick={() => set({ categoryIds: on ? s.categoryIds.filter((x) => x !== c.id) : [...s.categoryIds, c.id!] })}
                    className={cn('h-9 rounded-chip border px-3 text-sm font-semibold', on ? 'border-foreground bg-foreground text-background' : 'border-border bg-card')}>
                    {c.nameId}
                  </button>
                );
              })}
            </div>
          </fieldset>
        )}
        {needsImage(s.template) && (
          <div className="flex flex-col gap-2 sm:col-span-2">
            <span className="text-sm font-semibold">{t('admin.editor.photo')}</span>
            {s.imageKey && <img src={imageUrl(s.imageKey, 'sq640')} alt="" className="aspect-video w-full rounded-lg object-cover" />}
            <input ref={file} type="file" accept="image/jpeg,image/png,image/webp" hidden onChange={(e) => e.target.files?.[0] && void upload(e.target.files[0])} />
            <Button variant="secondary" className="self-start" disabled={uploading} onClick={() => file.current?.click()}>
              <ImagePlusIcon aria-hidden />
              {uploading ? t('admin.common.saving') : s.imageKey ? t('admin.editor.replacePhoto') : t('admin.editor.uploadPhoto')}
            </Button>
          </div>
        )}
        {s.template === 'uploaded' && (
          <div className="flex flex-col gap-2 sm:col-span-2">
            <span className="text-sm font-semibold">{t('admin.display.video')}</span>
            <p className="text-xs text-muted-foreground">{t('admin.display.videoHint')}</p>
            {s.videoKey && <video src={videoUrl(s.videoKey)} muted loop autoPlay playsInline className="aspect-video w-full rounded-lg bg-black object-cover" />}
            <input ref={video} type="file" accept="video/mp4,video/webm" hidden onChange={(e) => e.target.files?.[0] && void upload(e.target.files[0])} />
            <div className="flex gap-2">
              <Button variant="secondary" disabled={uploading} onClick={() => video.current?.click()}>
                <FilmIcon aria-hidden />
                {uploading ? t('admin.common.saving') : s.videoKey ? t('admin.display.replaceVideo') : t('admin.display.uploadVideo')}
              </Button>
              {s.videoKey && <Button variant="ghost" onClick={() => set({ videoKey: undefined })}>{t('admin.display.removeVideo')}</Button>}
            </div>
          </div>
        )}
        <Field label={t('admin.display.duration')} htmlFor="s-dur">
          <Input id="s-dur" type="number" min={4} max={60} className="tabular" value={s.durationSeconds} onChange={(e) => set({ durationSeconds: Number(e.target.value) })} />
        </Field>
        <div className="flex flex-col justify-end gap-3">
          {hasProduct && (
            <label className="flex items-center gap-3 text-sm font-semibold">
              <Switch checked={s.steam} onCheckedChange={(steam) => set({ steam })} />
              {t('admin.display.steam')}
            </label>
          )}
          <label className="flex items-center gap-3 text-sm font-semibold">
            <Switch checked={s.active} onCheckedChange={(active) => set({ active })} />
            {t('admin.common.active')}
          </label>
        </div>
      </div>
    </Dialog>
  );
}
